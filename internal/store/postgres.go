package store

import (
	"context"
	"database/sql"
	"embed"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"mood-tracker/internal/model"

	_ "github.com/lib/pq"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

//go:embed schema.sql
var schemaSQL string

//go:embed migrations/*.sql
var migrationsFS embed.FS

// timeFmt is how timestamps are stored as TEXT.
const timeFmt = time.RFC3339

type Postgres struct{ db *sql.DB }

// OpenPostgres opens a connection to the database and applies migrations.
// DSN format: postgres://user:pass@host:5432/dbname?sslmode=disable
func OpenPostgres(dsn string) (*Postgres, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		return nil, err
	}
	p := &Postgres{db: db}
	if err := p.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return p, nil
}

func (s *Postgres) migrate() error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	type migration struct {
		version int
		name    string
	}
	var migs []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		parts := strings.SplitN(e.Name(), "_", 2)
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		migs = append(migs, migration{version: v, name: e.Name()})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })

	for _, m := range migs {
		var exists bool
		err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, m.version).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + m.name)
		if err != nil {
			return err
		}
		if _, err := s.db.Exec(string(data)); err != nil {
			return fmt.Errorf("%s: %w", m.name, err)
		}
		if _, err := s.db.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES($1, $2)`,
			m.version, time.Now().UTC().Format(timeFmt)); err != nil {
			return err
		}
		log.Printf("applied migration %s", m.name)
	}
	return nil
}

func (s *Postgres) Close() error { return s.db.Close() }

// --- users ---

func (s *Postgres) CreateUser(ctx context.Context, email, hash, tz string) (*model.User, error) {
	now := time.Now().UTC()
	var id int64
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO users(email, password_hash, timezone, created_at)
		 VALUES($1,$2,$3,$4) RETURNING id`,
		email, hash, tz, now.Format(timeFmt)).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &model.User{ID: id, Email: email, PasswordHash: hash, Timezone: tz, CreatedAt: now}, nil
}

func (s *Postgres) UserByEmail(ctx context.Context, email string) (*model.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, timezone, COALESCE(username, '') FROM users WHERE email = $1`, email)
	return scanUser(row)
}

func (s *Postgres) UserByID(ctx context.Context, id int64) (*model.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, timezone, COALESCE(username, '') FROM users WHERE id = $1`, id)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*model.User, error) {
	var u model.User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Timezone, &u.Username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (s *Postgres) UpdateUsername(ctx context.Context, userID int64, username string) error {
	var uname *string
	if username != "" {
		uname = &username
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET username = $1 WHERE id = $2`, uname, userID)
	return err
}

func (s *Postgres) UpdatePassword(ctx context.Context, userID int64, hash string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`, hash, userID)
	return err
}

// --- sessions ---

func (s *Postgres) CreateSession(ctx context.Context, sess model.Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions(id, user_id, expires_at) VALUES($1,$2,$3)`,
		sess.ID, sess.UserID, sess.ExpiresAt.UTC().Format(timeFmt))
	return err
}

func (s *Postgres) SessionByID(ctx context.Context, id string) (*model.Session, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, expires_at FROM sessions WHERE id = $1`, id)
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

func (s *Postgres) DeleteSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}

func (s *Postgres) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at < $1`,
		time.Now().UTC().Format(timeFmt))
	return err
}

// --- mood entries ---

func (s *Postgres) UpsertMood(ctx context.Context, userID int64, date string, level int, note string) error {
	now := time.Now().UTC().Format(timeFmt)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO mood_entries(user_id, entry_date, mood_level, note, created_at, updated_at)
		 VALUES($1,$2,$3,$4,$5,$6)
		 ON CONFLICT(user_id, entry_date)
		 DO UPDATE SET mood_level = excluded.mood_level, note = excluded.note, updated_at = excluded.updated_at`,
		userID, date, level, note, now, now)
	return err
}

func (s *Postgres) TodayCount(ctx context.Context, date string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mood_entries WHERE entry_date = $1`, date).Scan(&n)
	return n, err
}

func (s *Postgres) MoodByUserAndDate(ctx context.Context, userID int64, date string) (*model.MoodEntry, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT user_id, entry_date, mood_level, COALESCE(note, '') FROM mood_entries
		 WHERE user_id = $1 AND entry_date = $2`,
		userID, date)
	var m model.MoodEntry
	if err := row.Scan(&m.UserID, &m.Date, &m.Level, &m.Note); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (s *Postgres) MoodsForUser(ctx context.Context, userID int64) ([]model.MoodEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, entry_date, mood_level, COALESCE(note, '') FROM mood_entries
		 WHERE user_id = $1 ORDER BY entry_date`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.MoodEntry
	for rows.Next() {
		var m model.MoodEntry
		if err := rows.Scan(&m.UserID, &m.Date, &m.Level, &m.Note); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Postgres) DailyAverages(ctx context.Context) ([]model.DayAverage, error) {
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
