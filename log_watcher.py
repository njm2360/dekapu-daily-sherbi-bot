import asyncio
import glob
import json
import logging
import os
import time
from pathlib import Path
from typing import IO, Awaitable, Callable, Optional

logger = logging.getLogger(__name__)

STATE_SAVE_INTERVAL = 60.0


class LogWatcher:
    def __init__(
        self,
        log_dir: str | Path,
        on_line: Callable[[str, str], Awaitable[None]],
        state_file: Optional[str | Path] = None,
        pattern: str = "output_log_*.txt",
        poll_interval: float = 1.0,
        scan_interval: float = 10.0,
        idle_timeout: float = 1800.0,
        read_from_end: bool = False,
    ) -> None:
        self.log_dir = Path(log_dir)
        self.on_line = on_line
        self.state_file = (
            Path(state_file) if state_file else self.log_dir / "state.json"
        )
        self.pattern = pattern
        self.poll_interval = poll_interval
        self.scan_interval = scan_interval
        self.idle_timeout = idle_timeout
        self.read_from_end = read_from_end

        self._offsets: dict[str, int] = {}
        self._load_state()

        self._handles: dict[str, IO[str]] = {}

        self._file_tasks: dict[str, Optional[asyncio.Task]] = {}

        self._scan_task: Optional[asyncio.Task] = None
        self._save_task: Optional[asyncio.Task] = None
        self._last_save = 0.0

    def _load_state(self) -> None:
        if self.state_file.exists():
            try:
                with self.state_file.open("r", encoding="utf-8") as f:
                    self._offsets = json.load(f)
            except (json.JSONDecodeError, OSError):
                self._offsets = {}

        existing = {name for name in self._offsets if (self.log_dir / name).exists()}
        removed = set(self._offsets) - existing
        for name in removed:
            del self._offsets[name]
            logger.info("Removed stale state entry: %s", name)

    def _save_state(self) -> None:
        tmp = self.state_file.with_suffix(".tmp")
        try:
            with tmp.open("w", encoding="utf-8") as f:
                json.dump(self._offsets, f, ensure_ascii=False, indent=2)
            tmp.replace(self.state_file)
            self._last_save = time.monotonic()
        except OSError:
            pass

    def _glob_files(self) -> list[str]:
        return sorted(glob.glob(str(self.log_dir / self.pattern)))

    def _open_handle(self, path: str) -> IO[str] | None:
        try:
            f = open(path, "r", encoding="utf-8", errors="replace")
            key = os.path.basename(path)
            if key in self._offsets:
                f.seek(self._offsets[key])
            elif self.read_from_end:
                f.seek(0, 2)  # ファイル末尾
            return f
        except OSError:
            return None

    def _close_handle(self, path: str) -> None:
        f = self._handles.pop(path, None)
        if f:
            try:
                f.close()
            except OSError:
                pass

    def _close_all_handles(self) -> None:
        for path in list(self._handles):
            self._close_handle(path)

    def _validate_handle(self, path: str, f: IO[str]) -> bool:
        try:
            size = os.path.getsize(path)
        except OSError:
            return False
        return size >= f.tell()

    def _read_lines(self, path: str) -> list[str]:
        if path not in self._handles:
            f = self._open_handle(path)
            if f is None:
                return []
            self._handles[path] = f

        f = self._handles[path]

        if not self._validate_handle(path, f):
            self._close_handle(path)
            self._offsets.pop(os.path.basename(path), None)
            f = self._open_handle(path)
            if f is None:
                return []
            self._handles[path] = f

        lines: list[str] = []
        try:
            for raw_line in f:
                lines.append(raw_line.rstrip("\n"))
            if lines:
                self._offsets[os.path.basename(path)] = f.tell()
        except OSError:
            self._close_handle(path)

        return lines

    async def _watch_file(self, path: str) -> None:
        last_active = time.monotonic()

        while True:
            lines = self._read_lines(path)

            if lines:
                last_active = time.monotonic()
                for line in lines:
                    await self.on_line(path, line)
            elif time.monotonic() - last_active >= self.idle_timeout:
                self._file_tasks[path] = None
                self._close_handle(path)
                logger.info(
                    "File is stale. Remove from monitoring task: %s", path
                )
                return

            await asyncio.sleep(self.poll_interval)

    async def _scan_loop(self) -> None:
        while True:
            for path in self._glob_files():
                if path not in self._file_tasks:
                    saved_offset = self._offsets.get(os.path.basename(path), 0)
                    if saved_offset > 0:
                        try:
                            current_size = os.path.getsize(path)
                        except OSError:
                            current_size = 0
                        if current_size <= saved_offset:
                            self._file_tasks[path] = None
                            continue
                    task = asyncio.create_task(
                        self._watch_file(path), name=f"watch:{path}"
                    )
                    self._file_tasks[path] = task
                    logger.info(
                        "Monitoring start: %s", path
                    )

            for path, task in list(self._file_tasks.items()):
                if task is not None:
                    continue
                try:
                    current_size = os.path.getsize(path)
                except OSError:
                    continue
                if current_size > self._offsets.get(os.path.basename(path), 0):
                    self._file_tasks[path] = asyncio.create_task(
                        self._watch_file(path), name=f"watch:{path}"
                    )
                    logger.info("Monitoring resume: %s", path)

            await asyncio.sleep(self.scan_interval)

    async def _state_save_loop(self) -> None:
        while True:
            await asyncio.sleep(STATE_SAVE_INTERVAL)
            self._save_state()

    async def run(self) -> None:
        self._last_save = time.monotonic()

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
                *[t for t in self._file_tasks.values() if t is not None],
            ]
            for t in all_tasks:
                t.cancel()
            await asyncio.gather(*all_tasks, return_exceptions=True)
            self._close_all_handles()
            self._save_state()

    def stop(self) -> None:
        if self._scan_task:
            self._scan_task.cancel()
        if self._save_task:
            self._save_task.cancel()
