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
//	go run ./cmd/seed -docker         # seed the running docker compose db
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
	"os/exec"
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

const dockerDSN = "postgres://mood:mood@postgres:5432/mood?sslmode=disable"

func main() {
	n := flag.Int("n", 100, "number of test accounts to create")
	days := flag.Int("days", 365, "how many days of history to backfill")
	password := flag.String("password", "password123", "shared password for all seeded accounts")
	seed := flag.Int64("seed", time.Now().UnixNano(), "RNG seed (set for reproducible data)")
	docker := flag.Bool("docker", false, "seed the database running in docker compose")
	flag.Parse()

	if *docker {
		seedInDocker()
		return
	}

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://mood:mood@localhost:5432/mood?sslmode=disable"
	}

	st, err := store.OpenPostgres(dsn)
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

			if err := st.UpsertMood(ctx, u.ID, date, level, ""); err != nil {
				log.Fatalf("upsert mood for %s on %s: %v", email, date, err)
			}
			totalEntries++
		}
	}

	log.Printf("seed complete: %d users created, %d existing reused, %d mood entries written",
		createdUsers, skipped, totalEntries)
	log.Printf("log in as any of user001@example.com .. user%03d@example.com (password: %q)", *n, *password)
}

// seedInDocker builds a Linux binary from the current source, copies it into
// the running docker compose "app" container, and execs it there against the
// container's database. All flags except -docker are forwarded as-is.
func seedInDocker() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("getwd: %v", err)
	}

	tmp, err := os.CreateTemp("", "mood-seed-*")
	if err != nil {
		log.Fatalf("tempfile: %v", err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	log.Println("seed: building linux binary…")
	build := exec.Command("go", "build", "-o", tmp.Name(), "./cmd/seed")
	build.Dir = cwd
	build.Env = append(os.Environ(), "GOOS=linux", "CGO_ENABLED=0")
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		log.Fatalf("build: %v", err)
	}

	log.Println("seed: copying binary into container…")
	cp := exec.Command("docker", "compose", "cp", tmp.Name(), "app:/tmp/mood-seed")
	cp.Dir = cwd
	cp.Stdout = os.Stderr
	cp.Stderr = os.Stderr
	if err := cp.Run(); err != nil {
		log.Fatalf("docker compose cp: %v", err)
	}

	// Forward all flags except -docker / --docker.
	var inner []string
	for _, a := range os.Args[1:] {
		if a == "-docker" || a == "--docker" {
			continue
		}
		inner = append(inner, a)
	}

	runArgs := []string{"compose", "exec", "-T", "-e", "DB_DSN=" + dockerDSN, "app", "/tmp/mood-seed"}
	runArgs = append(runArgs, inner...)
	cmd := exec.Command("docker", runArgs...)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("seed in container: %v", err)
	}
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
