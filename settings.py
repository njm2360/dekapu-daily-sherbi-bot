import sqlite3
import logging
from dataclasses import dataclass
from pathlib import Path

from ball_parser import Lang

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class ChannelConfig:
    guild_id: str
    channel_id: int
    lang: Lang


_SCHEMA = """
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS guilds (
    guild_id  TEXT PRIMARY KEY,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS notification_channels (
    guild_id   TEXT    NOT NULL REFERENCES guilds(guild_id) ON DELETE CASCADE,
    channel_id INTEGER NOT NULL,
    lang       TEXT    NOT NULL DEFAULT 'ja',
    updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (guild_id, channel_id)
);

CREATE TABLE IF NOT EXISTS guild_settings (
    guild_id   TEXT NOT NULL REFERENCES guilds(guild_id) ON DELETE CASCADE,
    key        TEXT NOT NULL,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (guild_id, key)
);
"""


class SettingsRepository:
    def __init__(self, db_path: str | Path) -> None:
        self._conn = sqlite3.connect(str(db_path), check_same_thread=False)
        self._conn.row_factory = sqlite3.Row
        self._migrate()

    def _migrate(self) -> None:
        self._conn.executescript(_SCHEMA)
        self._conn.commit()

    # ------------------------------------------------------------------
    # 通知チャンネル
    # ------------------------------------------------------------------

    def get_all_channels(self) -> list[ChannelConfig]:
        rows = self._conn.execute(
            "SELECT guild_id, channel_id, lang FROM notification_channels"
        ).fetchall()
        return [
            ChannelConfig(row["guild_id"], row["channel_id"], Lang(row["lang"]))
            for row in rows
        ]

    def get_channels(self, guild_id: str) -> list[int]:
        rows = self._conn.execute(
            "SELECT channel_id FROM notification_channels WHERE guild_id = ?",
            (guild_id,),
        ).fetchall()
        return [row["channel_id"] for row in rows]

    def set_channel(self, guild_id: str, channel_id: int, lang: Lang = Lang.JA) -> None:
        with self._conn:
            self._conn.execute(
                """
                INSERT INTO notification_channels(guild_id, channel_id, lang, updated_at)
                     VALUES (?, ?, ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
                ON CONFLICT(guild_id, channel_id) DO UPDATE
                    SET lang       = excluded.lang,
                        updated_at = excluded.updated_at
                """,
                (guild_id, channel_id, lang.value),
            )

    def unset_channel(self, guild_id: str, channel_id: int) -> bool:
        with self._conn:
            cur = self._conn.execute(
                "DELETE FROM notification_channels WHERE guild_id = ? AND channel_id = ?",
                (guild_id, channel_id),
            )
        return cur.rowcount > 0

    # ------------------------------------------------------------------
    # 汎用 KV ストア
    # ------------------------------------------------------------------

    def get_setting(
        self, guild_id: str, key: str, default: str | None = None
    ) -> str | None:
        row = self._conn.execute(
            "SELECT value FROM guild_settings WHERE guild_id = ? AND key = ?",
            (guild_id, key),
        ).fetchone()
        return row["value"] if row else default

    def set_setting(self, guild_id: str, key: str, value: str) -> None:
        with self._conn:
            self._conn.execute(
                """
                INSERT INTO guild_settings(guild_id, key, value, updated_at)
                     VALUES (?, ?, ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
                ON CONFLICT(guild_id, key) DO UPDATE
                    SET value      = excluded.value,
                        updated_at = excluded.updated_at
                """,
                (guild_id, key, value),
            )

    def delete_setting(self, guild_id: str, key: str) -> bool:
        with self._conn:
            cur = self._conn.execute(
                "DELETE FROM guild_settings WHERE guild_id = ? AND key = ?",
                (guild_id, key),
            )
        return cur.rowcount > 0

    # ------------------------------------------------------------------
    # メンションロール
    # ------------------------------------------------------------------

    _MENTION_ROLE_KEY = "mention_role_id"

    def get_mention_role(self, guild_id: str) -> int | None:
        value = self.get_setting(guild_id, self._MENTION_ROLE_KEY)
        return int(value) if value is not None else None

    def set_mention_role(self, guild_id: str, role_id: int) -> None:
        self.set_setting(guild_id, self._MENTION_ROLE_KEY, str(role_id))

    def unset_mention_role(self, guild_id: str) -> bool:
        return self.delete_setting(guild_id, self._MENTION_ROLE_KEY)

    def ensure_guild(self, guild_id: str) -> None:
        with self._conn:
            self._conn.execute(
                "INSERT INTO guilds(guild_id) VALUES (?) ON CONFLICT DO NOTHING",
                (guild_id,),
            )

    def remove_guild(self, guild_id: str) -> None:
        with self._conn:
            self._conn.execute("DELETE FROM guilds WHERE guild_id = ?", (guild_id,))

    def close(self) -> None:
        self._conn.close()
