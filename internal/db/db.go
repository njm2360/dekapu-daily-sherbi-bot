package db

import (
	"database/sql"
	_ "embed"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// Open opens the SQLite file at path, applies PRAGMAs and the full schema,
// and returns a connection-pool-limited *sql.DB. The caller owns the handle.
func Open(path string) (*sql.DB, error) {
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1)
	if _, err := d.Exec(schema); err != nil {
		_ = d.Close()
		return nil, err
	}
	return d, nil
}
