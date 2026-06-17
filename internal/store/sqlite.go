package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"time"

	"mood-tracker/internal/model"

	_ "modernc.org/sqlite" // pure-Go driver, registered as "sqlite"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

//go:embed schema.sql
var schemaSQL string

// timeFmt is how timestamps are stored as TEXT, to avoid driver-specific
// datetime handling. Dates (entry_date) are stored as plain "YYYY-MM-DD".
const timeFmt = time.RFC3339

type SQLite struct{ db *sql.DB }

// OpenSQLite opens (creating if needed) the database and applies the schema.
// A good DSN sets WAL + busy_timeout, e.g.:
//
//	file:mood.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)
func OpenSQLite(dsn string) (*SQLite, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		return nil, err
	}
	return &SQLite{db: db}, nil
}

func (s *SQLite) Close() error { return s.db.Close() }

// --- users ---

func (s *SQLite) CreateUser(ctx context.Context, email, hash, tz string) (*model.User, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users(email, password_hash, timezone, created_at) VALUES(?,?,?,?)`,
		email, hash, tz, now.Format(timeFmt))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &model.User{ID: id, Email: email, PasswordHash: hash, Timezone: tz, CreatedAt: now}, nil
}

func (s *SQLite) UserByEmail(ctx context.Context, email string) (*model.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, timezone FROM users WHERE email = ?`, email)
	return scanUser(row)
}

func (s *SQLite) UserByID(ctx context.Context, id int64) (*model.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, timezone FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*model.User, error) {
	var u model.User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Timezone); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// --- sessions ---

func (s *SQLite) CreateSession(ctx context.Context, sess model.Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions(id, user_id, expires_at) VALUES(?,?,?)`,
		sess.ID, sess.UserID, sess.ExpiresAt.UTC().Format(timeFmt))
	return err
}

func (s *SQLite) SessionByID(ctx context.Context, id string) (*model.Session, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, expires_at FROM sessions WHERE id = ?`, id)
	var sess model.Session
	var exp string
	if err := row.Scan(&sess.ID, &sess.UserID, &exp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	sess.ExpiresAt, _ = time.Parse(timeFmt, exp)
	return &sess, nil
}

func (s *SQLite) DeleteSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func (s *SQLite) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at < ?`,
		time.Now().UTC().Format(timeFmt))
	return err
}

// --- mood entries ---

func (s *SQLite) UpsertMood(ctx context.Context, userID int64, date string, level int) error {
	now := time.Now().UTC().Format(timeFmt)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO mood_entries(user_id, entry_date, mood_level, created_at, updated_at)
		 VALUES(?,?,?,?,?)
		 ON CONFLICT(user_id, entry_date)
		 DO UPDATE SET mood_level = excluded.mood_level, updated_at = excluded.updated_at`,
		userID, date, level, now, now)
	return err
}

func (s *SQLite) MoodByUserAndDate(ctx context.Context, userID int64, date string) (*model.MoodEntry, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT user_id, entry_date, mood_level FROM mood_entries WHERE user_id = ? AND entry_date = ?`,
		userID, date)
	var m model.MoodEntry
	if err := row.Scan(&m.UserID, &m.Date, &m.Level); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (s *SQLite) MoodsForUser(ctx context.Context, userID int64) ([]model.MoodEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, entry_date, mood_level FROM mood_entries WHERE user_id = ? ORDER BY entry_date`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.MoodEntry
	for rows.Next() {
		var m model.MoodEntry
		if err := rows.Scan(&m.UserID, &m.Date, &m.Level); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *SQLite) DailyAverages(ctx context.Context) ([]model.DayAverage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT entry_date, AVG(mood_level), COUNT(*)
		 FROM mood_entries GROUP BY entry_date ORDER BY entry_date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.DayAverage
	for rows.Next() {
		var a model.DayAverage
		if err := rows.Scan(&a.Date, &a.Avg, &a.Count); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
