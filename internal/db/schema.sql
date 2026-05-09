PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS guilds (
    guild_id   TEXT PRIMARY KEY,
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

CREATE TABLE IF NOT EXISTS daily_history (
    day_number  INTEGER NOT NULL,
    revision    INTEGER NOT NULL DEFAULT 0,
    detected_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (day_number, revision)
);

CREATE TABLE IF NOT EXISTS daily_history_balls (
    day_number INTEGER NOT NULL,
    revision   INTEGER NOT NULL,
    position   INTEGER NOT NULL,
    ball_id    INTEGER NOT NULL,
    PRIMARY KEY (day_number, revision, position),
    FOREIGN KEY (day_number, revision)
        REFERENCES daily_history(day_number, revision) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_daily_history_detected_at ON daily_history(detected_at);
CREATE INDEX IF NOT EXISTS idx_daily_history_balls_ball_day ON daily_history_balls(ball_id, day_number DESC);
