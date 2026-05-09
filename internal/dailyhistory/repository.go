package dailyhistory

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
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

// Match represents one (day_number, revision) entry whose ball set
// fully contained the queried ball IDs, with that revision's full ball list.
type Match struct {
	DayNumber int
	Revision  int
	BallIDs   []int
}

// FindByBalls returns up to `limit` most recent days where any revision's
// ball set contained ALL of the given ballIDs (set match, order independent).
// Returned matches may include multiple revisions for the same day.
// Results are ordered by day_number DESC, then revision ASC.
func (r *Repository) FindByBalls(ballIDs []int, limit int) ([]Match, error) {
	if len(ballIDs) == 0 || limit <= 0 {
		return nil, nil
	}

	// dedup the input ball IDs to keep COUNT(DISTINCT ball_id) = N consistent
	seen := make(map[int]struct{}, len(ballIDs))
	uniq := make([]int, 0, len(ballIDs))
	for _, id := range ballIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(uniq)), ",")
	query := fmt.Sprintf(`
        WITH matching AS (
          SELECT day_number, revision
            FROM daily_history_balls
           WHERE ball_id IN (%s)
           GROUP BY day_number, revision
          HAVING COUNT(DISTINCT ball_id) = ?
        ),
        top_days AS (
          SELECT DISTINCT day_number FROM matching
           ORDER BY day_number DESC LIMIT ?
        )
        SELECT m.day_number, m.revision, b.position, b.ball_id
          FROM matching m
          JOIN top_days t ON m.day_number = t.day_number
          JOIN daily_history_balls b
               ON b.day_number = m.day_number AND b.revision = m.revision
         ORDER BY m.day_number DESC, m.revision ASC, b.position ASC
    `, placeholders)

	args := make([]any, 0, len(uniq)+2)
	for _, id := range uniq {
		args = append(args, id)
	}
	args = append(args, len(uniq), limit)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []Match
	var current *Match
	for rows.Next() {
		var day, rev, pos, ballID int
		if err := rows.Scan(&day, &rev, &pos, &ballID); err != nil {
			return nil, err
		}
		if current == nil || current.DayNumber != day || current.Revision != rev {
			matches = append(matches, Match{DayNumber: day, Revision: rev})
			current = &matches[len(matches)-1]
		}
		current.BallIDs = append(current.BallIDs, ballID)
	}
	return matches, rows.Err()
}
