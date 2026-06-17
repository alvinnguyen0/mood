// Command seed populates the database with test accounts and varied mood
// history, so the "Everyone" grid (and individual "You" grids) have something
// interesting to render in development.
//
// It reuses the real store + schema, so seeded users can log in normally.
//
// Usage:
//
//	go run ./cmd/seed                 # 100 users into the default mood.db
//	go run ./cmd/seed -n 50 -seed 7   # 50 users, reproducible RNG
//	DB_DSN=file:dev.db go run ./cmd/seed
//
// All seeded accounts share the same password (default "password123") so you
// can log in as any of them. Emails are user001@example.com .. userNNN@...
//
// Running it again is safe: users that already exist are skipped, and mood
// entries upsert, so the data converges rather than duplicating.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"time"

	"mood-tracker/internal/store"

	"golang.org/x/crypto/bcrypt"
)

const dateFmt = "2006-01-02"

// timezones spread the accounts across a few IANA zones so the "today"/local
// date logic gets exercised, not just UTC.
var timezones = []string{
	"UTC",
	"America/New_York",
	"America/Los_Angeles",
	"Europe/London",
	"Europe/Berlin",
	"Asia/Tokyo",
	"Australia/Sydney",
}

func main() {
	n := flag.Int("n", 100, "number of test accounts to create")
	days := flag.Int("days", 365, "how many days of history to backfill")
	password := flag.String("password", "password123", "shared password for all seeded accounts")
	seed := flag.Int64("seed", time.Now().UnixNano(), "RNG seed (set for reproducible data)")
	flag.Parse()

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "file:mood.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	}

	st, err := store.OpenSQLite(dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer st.Close()

	// Hash the shared password once; bcrypt is intentionally slow, so doing it
	// per-user would dominate the runtime for no benefit.
	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	rng := rand.New(rand.NewSource(*seed))
	ctx := context.Background()

	// Per-day community mood drift: a slow sine wave so the Everyone grid shows
	// genuinely good and bad stretches instead of uniform noise.
	dayBias := func(daysAgo int) float64 {
		return 0.8 * math.Sin(float64(daysAgo)/18.0)
	}

	var createdUsers, totalEntries, skipped int
	for i := 1; i <= *n; i++ {
		email := fmt.Sprintf("user%03d@example.com", i)
		tz := timezones[rng.Intn(len(timezones))]

		u, err := st.CreateUser(ctx, email, string(hash), tz)
		if err != nil {
			// Most likely the account already exists (UNIQUE email) from a
			// prior run — look it up and keep going so reseeding is idempotent.
			existing, lookupErr := st.UserByEmail(ctx, email)
			if lookupErr != nil {
				log.Fatalf("create/lookup user %s: create=%v lookup=%v", email, err, lookupErr)
			}
			u = existing
			skipped++
		} else {
			createdUsers++
		}

		// Each user has a baseline mood (some people run cheerful, some glum),
		// a logging frequency (some log most days, some are sporadic), and a
		// little day-to-day volatility.
		userBaseline := 2.0 + rng.Float64()*2.0 // ~2.0..4.0
		logProb := 0.4 + rng.Float64()*0.55     // ~40%..95% of days logged

		for d := 0; d < *days; d++ {
			if rng.Float64() > logProb {
				continue // user didn't log a mood that day
			}
			date := time.Now().UTC().AddDate(0, 0, -d).Format(dateFmt)

			val := userBaseline + dayBias(d) + (rng.Float64()*2.0 - 1.0)
			level := clampLevel(int(val + 0.5))

			if err := st.UpsertMood(ctx, u.ID, date, level); err != nil {
				log.Fatalf("upsert mood for %s on %s: %v", email, date, err)
			}
			totalEntries++
		}
	}

	log.Printf("seed complete: %d users created, %d existing reused, %d mood entries written",
		createdUsers, skipped, totalEntries)
	log.Printf("log in as any of user001@example.com .. user%03d@example.com (password: %q)", *n, *password)
}

// clampLevel keeps a computed mood within the valid 1..5 range.
func clampLevel(v int) int {
	if v < 1 {
		return 1
	}
	if v > 5 {
		return 5
	}
	return v
}
