package model

import "time"

// User is an account. Entries are private to the user; the Everyone view
// only ever exposes aggregates.
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Timezone     string // IANA name, e.g. "America/New_York"
	CreatedAt    time.Time
}

// MoodEntry is one logged mood. One per user per day, keyed by the user's
// local calendar date (Date, "YYYY-MM-DD").
type MoodEntry struct {
	UserID int64
	Date   string
	Level  int // 1..5
}

// Session is a server-side login session; the opaque ID lives in a cookie.
type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
}

// DayAverage is one cell of the Everyone grid: the average mood across all
// users for a given date, plus how many people logged it.
type DayAverage struct {
	Date  string
	Avg   float64
	Count int
}
