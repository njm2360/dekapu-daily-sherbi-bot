import time
import json
import logging
import asyncio
from pathlib import Path
from typing import Awaitable, Callable, Optional

logger = logging.getLogger(__name__)

STATE_SAVE_INTERVAL = 60.0


class WatcherState:
    def __init__(self, state_file: Path) -> None:
        self._state_file = state_file

    def load(self) -> dict[Path, int]:
        if not self._state_file.exists():
            return {}
        try:
            with self._state_file.open("r", encoding="utf-8") as f:
                return {Path(k): v for k, v in json.load(f).items()}
        except (json.JSONDecodeError, OSError):
            return {}

    def save(self, offsets: dict[Path, int]) -> None:
        tmp = self._state_file.with_suffix(".tmp")
        try:
            with tmp.open("w", encoding="utf-8") as f:
                json.dump(
                    {str(k): v for k, v in offsets.items()},
                    f,
                    ensure_ascii=False,
                    indent=2,
                )
            tmp.replace(self._state_file)
        except OSError:
            logger.warning("Failed to save state: %s", self._state_file, exc_info=True)


class LogWatcher:
    def __init__(
        self,
        log_dir: Path,
        on_line: Callable[[Path, str], Awaitable[None]],
        state_file: Optional[str | Path] = None,
        pattern: str = "output_log_*.txt",
        poll_interval: float = 1.0,
        scan_interval: float = 10.0,
        idle_timeout: float = 1800.0,
        read_from_end: bool = False,
    ) -> None:
        self.log_dir = log_dir
        self.on_line = on_line
        self.pattern = pattern
        self.poll_interval = poll_interval
        self.scan_interval = scan_interval
        self.idle_timeout = idle_timeout
        self.read_from_end = read_from_end

        state_path = Path(state_file) if state_file else self.log_dir / "state.json"
        self._state = WatcherState(state_path)
        self._offsets: dict[Path, int] = self._state.load()

        stale = [p for p in self._offsets if not p.exists()]
        for path in stale:
            del self._offsets[path]
            logger.info("Removed stale state entry: %s", path)

        self._known_files: set[Path] = set()
        self._watch_tasks: dict[Path, asyncio.Task] = {}

        self._scan_task: Optional[asyncio.Task] = None
        self._save_task: Optional[asyncio.Task] = None

    async def _watch_file(self, path: Path) -> None:
        last_active = time.monotonic()

        with path.open("r", encoding="utf-8", errors="replace") as f:
            if path in self._offsets:
                f.seek(self._offsets[path])
            elif self.read_from_end:
                f.seek(0, 2)
                self._offsets[path] = f.tell()

            while True:
                lines = []
                while True:
                    pos = f.tell()
                    line = f.readline()
                    if not line:
                        break
                    if not line.endswith("\n"):
                        # 不完全な行は次回に持ち越すために位置を戻す
                        f.seek(pos)
                        break
                    lines.append(line.rstrip("\n"))
                if lines:
                    last_active = time.monotonic()
                    self._offsets[path] = f.tell()
                    for line in lines:
                        try:
                            await self.on_line(path, line)
                        except Exception:
                            logger.exception("on_line raised an error: %s", path)
                elif time.monotonic() - last_active >= self.idle_timeout:
                    logger.info("File is stale. Remove from monitoring task: %s", path)
                    return
                await asyncio.sleep(self.poll_interval)

    def _start_watch_task(self, path: Path) -> None:
        def _on_done(task: asyncio.Task, p: Path = path) -> None:
            self._watch_tasks.pop(p, None)
            if not task.cancelled() and (exc := task.exception()):
                logger.warning("Watch task error for %s: %s", p, exc)

        task = asyncio.create_task(self._watch_file(path), name=f"watch:{path}")
        task.add_done_callback(_on_done)
        self._known_files.add(path)
        self._watch_tasks[path] = task

    async def _scan_loop(self) -> None:
        first_scan = True
        while True:
            for path in self.log_dir.glob(self.pattern):
                if path not in self._known_files:
                    # 起動後に新規作成されたファイルは先頭から読む
                    if not first_scan and path not in self._offsets:
                        self._offsets[path] = 0
                    saved_offset = self._offsets.get(path, 0)
                    if saved_offset > 0:
                        try:
                            current_size = path.stat().st_size
                        except OSError:
                            continue
                        if current_size <= saved_offset:  # 追記無しは待機
                            self._known_files.add(path)
                            continue
                    self._start_watch_task(path)
                    logger.info("Monitoring start: %s", path)

            for path in list(self._known_files):
                if path in self._watch_tasks:
                    continue
                try:
                    current_size = path.stat().st_size
                except OSError:
                    self._known_files.discard(path)
                    continue
                if current_size > self._offsets.get(path, 0):
                    self._start_watch_task(path)
                    logger.info("Monitoring resume: %s", path)

            first_scan = False
            await asyncio.sleep(self.scan_interval)

    async def _state_save_loop(self) -> None:
        while True:
            await asyncio.sleep(STATE_SAVE_INTERVAL)
            self._state.save(self._offsets)

    async def run(self) -> None:
        self._scan_task = asyncio.create_task(self._scan_loop(), name="scan")
        self._save_task = asyncio.create_task(
            self._state_save_loop(), name="state_save"
        )

        try:
            await asyncio.gather(self._scan_task, self._save_task)
        except asyncio.CancelledError:
            pass
        finally:
            all_tasks = [
                self._scan_task,
                self._save_task,
                *self._watch_tasks.values(),
            ]
            for t in all_tasks:
                t.cancel()
            await asyncio.gather(*all_tasks, return_exceptions=True)
            self._state.save(self._offsets)

    def stop(self) -> None:
        if self._scan_task:
            self._scan_task.cancel()
        if self._save_task:
            self._save_task.cancel()
        for task in self._watch_tasks.values():
            task.cancel()
