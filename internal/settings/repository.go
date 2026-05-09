package settings

import (
	"database/sql"
	"errors"
	"strconv"

	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/ball"
)

const mentionRoleKey = "mention_role_id"

type ChannelConfig struct {
	GuildID   string
	ChannelID int64
	Lang      ball.Lang
}

type Repository struct {
	db *sql.DB
}

// New returns a Repository backed by db. Schema is expected to be applied
// by the caller (see internal/db.Open).
func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// ---- guilds ----

func (r *Repository) EnsureGuild(guildID string) error {
	_, err := r.db.Exec(
		"INSERT INTO guilds(guild_id) VALUES (?) ON CONFLICT DO NOTHING",
		guildID,
	)
	return err
}

func (r *Repository) RemoveGuild(guildID string) error {
	_, err := r.db.Exec("DELETE FROM guilds WHERE guild_id = ?", guildID)
	return err
}

// ---- notification channels ----

func (r *Repository) AllChannels() ([]ChannelConfig, error) {
	rows, err := r.db.Query("SELECT guild_id, channel_id, lang FROM notification_channels")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChannelConfig
	for rows.Next() {
		var c ChannelConfig
		var lang string
		if err := rows.Scan(&c.GuildID, &c.ChannelID, &lang); err != nil {
			return nil, err
		}
		c.Lang = ball.Lang(lang)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) Channels(guildID string) ([]int64, error) {
	rows, err := r.db.Query(
		"SELECT channel_id FROM notification_channels WHERE guild_id = ?",
		guildID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *Repository) SetChannel(guildID string, channelID int64, lang ball.Lang) error {
	_, err := r.db.Exec(`
        INSERT INTO notification_channels(guild_id, channel_id, lang, updated_at)
             VALUES (?, ?, ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        ON CONFLICT(guild_id, channel_id) DO UPDATE
            SET lang       = excluded.lang,
                updated_at = excluded.updated_at
    `, guildID, channelID, string(lang))
	return err
}

func (r *Repository) UnsetChannel(guildID string, channelID int64) (bool, error) {
	res, err := r.db.Exec(
		"DELETE FROM notification_channels WHERE guild_id = ? AND channel_id = ?",
		guildID, channelID,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// ---- generic KV ----

func (r *Repository) GetSetting(guildID, key string) (string, bool, error) {
	var v string
	err := r.db.QueryRow(
		"SELECT value FROM guild_settings WHERE guild_id = ? AND key = ?",
		guildID, key,
	).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (r *Repository) SetSetting(guildID, key, value string) error {
	_, err := r.db.Exec(`
        INSERT INTO guild_settings(guild_id, key, value, updated_at)
             VALUES (?, ?, ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        ON CONFLICT(guild_id, key) DO UPDATE
            SET value      = excluded.value,
                updated_at = excluded.updated_at
    `, guildID, key, value)
	return err
}

func (r *Repository) DeleteSetting(guildID, key string) (bool, error) {
	res, err := r.db.Exec(
		"DELETE FROM guild_settings WHERE guild_id = ? AND key = ?",
		guildID, key,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// ---- mention role (KV wrapper) ----

func (r *Repository) GetMentionRole(guildID string) (int64, bool, error) {
	v, ok, err := r.GetSetting(guildID, mentionRoleKey)
	if err != nil || !ok {
		return 0, ok, err
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func (r *Repository) SetMentionRole(guildID string, roleID int64) error {
	return r.SetSetting(guildID, mentionRoleKey, strconv.FormatInt(roleID, 10))
}

func (r *Repository) UnsetMentionRole(guildID string) (bool, error) {
	return r.DeleteSetting(guildID, mentionRoleKey)
}
