package settings

import (
	"database/sql"
	"errors"

	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/language"
)

const mentionRoleKey = "mention_role_id"

type ChannelConfig struct {
	GuildID   string
	ChannelID string
	Lang      language.Lang
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
		var langStr string
		if err := rows.Scan(&c.GuildID, &c.ChannelID, &langStr); err != nil {
			return nil, err
		}
		c.Lang = language.Lang(langStr)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) Channels(guildID string) ([]string, error) {
	rows, err := r.db.Query(
		"SELECT channel_id FROM notification_channels WHERE guild_id = ?",
		guildID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *Repository) SetChannel(guildID, channelID string, lang language.Lang) error {
	_, err := r.db.Exec(`
        INSERT INTO notification_channels(guild_id, channel_id, lang, updated_at)
             VALUES (?, ?, ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        ON CONFLICT(guild_id, channel_id) DO UPDATE
            SET lang       = excluded.lang,
                updated_at = excluded.updated_at
    `, guildID, channelID, string(lang))
	return err
}

func (r *Repository) UnsetChannel(guildID, channelID string) (bool, error) {
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

func (r *Repository) GetMentionRole(guildID string) (string, bool, error) {
	return r.GetSetting(guildID, mentionRoleKey)
}

func (r *Repository) SetMentionRole(guildID, roleID string) error {
	return r.SetSetting(guildID, mentionRoleKey, roleID)
}

func (r *Repository) UnsetMentionRole(guildID string) (bool, error) {
	return r.DeleteSetting(guildID, mentionRoleKey)
}
