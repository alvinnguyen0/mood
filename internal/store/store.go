package store

import (
	"context"

	"mood-tracker/internal/model"
)

// Store is the persistence boundary. The rest of the app depends only on this
// interface, so swapping SQLite for Postgres later is a new implementation,
// not a rewrite.
type Store interface {
	// Users
	CreateUser(ctx context.Context, email, passwordHash, timezone string) (*model.User, error)
	UserByEmail(ctx context.Context, email string) (*model.User, error)
	UserByID(ctx context.Context, id int64) (*model.User, error)

	// Sessions
	CreateSession(ctx context.Context, s model.Session) error
	SessionByID(ctx context.Context, id string) (*model.Session, error)
	DeleteSession(ctx context.Context, id string) error
	DeleteExpiredSessions(ctx context.Context) error

	// Mood entries
	UpsertMood(ctx context.Context, userID int64, date string, level int) error
	MoodByUserAndDate(ctx context.Context, userID int64, date string) (*model.MoodEntry, error)
	MoodsForUser(ctx context.Context, userID int64) ([]model.MoodEntry, error)
	DailyAverages(ctx context.Context) ([]model.DayAverage, error)

	Close() error
}

// Compile-time check that the SQLite implementation satisfies Store.
var _ Store = (*SQLite)(nil)
