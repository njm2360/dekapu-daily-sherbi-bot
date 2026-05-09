package dailyhistory

import (
	"database/sql"
	"errors"
	"time"
)

type Record struct {
	DayNumber  int
	Revision   int
	BallIDs    []int
	DetectedAt time.Time
}

type Repository struct {
	db *sql.DB
}

// New returns a Repository backed by db. Schema is expected to be applied
// by the caller (see internal/db.Open).
func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Latest returns the most recently detected record across all days,
// or (nil, nil) when the history is empty.
func (r *Repository) Latest() (*Record, error) {
	var rec Record
	var detectedAt string
	err := r.db.QueryRow(`
        SELECT day_number, revision, detected_at
          FROM daily_history
         ORDER BY day_number DESC, revision DESC
         LIMIT 1
    `).Scan(&rec.DayNumber, &rec.Revision, &detectedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if t, err := time.Parse("2006-01-02T15:04:05Z", detectedAt); err == nil {
		rec.DetectedAt = t
	}

	rows, err := r.db.Query(`
        SELECT ball_id FROM daily_history_balls
         WHERE day_number = ? AND revision = ?
         ORDER BY position
    `, rec.DayNumber, rec.Revision)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		rec.BallIDs = append(rec.BallIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &rec, nil
}

// Insert writes a daily_history row plus its ball positions atomically.
func (r *Repository) Insert(dayNumber, revision int, ballIDs []int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO daily_history(day_number, revision) VALUES (?, ?)`,
		dayNumber, revision,
	); err != nil {
		return err
	}
	stmt, err := tx.Prepare(
		`INSERT INTO daily_history_balls(day_number, revision, position, ball_id) VALUES (?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, id := range ballIDs {
		if _, err := stmt.Exec(dayNumber, revision, i, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
