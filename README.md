# mood

A minimalist mood tracker. Log one mood a day (five faces, 🙁 → 😄), see your
year as a GitHub-style activity grid with current/longest streaks, and view an
**Everyone** tab showing the daily average across all users. Individual entries
stay private; the community view only ever exposes aggregates.

Server-rendered Go (no SPA, no build step). Stack per the plan: **chi** router,
**SQLite** via the pure-Go `modernc.org/sqlite` driver, **bcrypt** password
hashing, `html/template` + **htmx** for the one bit of interactivity (logging a
mood swaps just the picker card; it also works without JS via a normal form post).

## Run

Requires Go 1.22+ and network access the first time (to fetch dependencies).

```bash
cd mood-tracker
go mod tidy      # resolves chi, x/crypto, modernc.org/sqlite
go run .
```

Then open http://localhost:8080 — sign up, and start logging.

Config via env vars:

- `ADDR` — listen address (default `:8080`)
- `DB_DSN` — SQLite DSN (default `file:mood.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)`)

## Seed test data

`cmd/seed` populates the database with test accounts and ~a year of varied mood
history, so the **Everyone** grid (and individual **You** grids) have something
to render. It writes to the same DB as the server (`DB_DSN`, default `mood.db`).

```bash
go run ./cmd/seed                  # 100 accounts, ~365 days of history
go run ./cmd/seed -n 50 -seed 7    # 50 accounts, reproducible RNG
DB_DSN=file:dev.db go run ./cmd/seed
```

Accounts are `user001@example.com` … `userNNN@example.com`, all sharing one
password (default `password123`, override with `-password`), so you can log in
as any of them. Moods vary per-user (cheerful vs. glum baselines, sparse vs.
diligent loggers) over a slow community drift, so days read as genuinely good or
bad rather than uniform noise. Re-running is idempotent: existing users are
skipped and mood entries upsert, so the data converges instead of duplicating.

Flags: `-n` (accounts), `-days` (history depth), `-password`, `-seed`.

> **Not yet compiled/tested in this environment** — it was written without a Go
> toolchain or network available, so `go run .` is the first real build. If the
> compiler flags anything, it'll be a small fix; the structure and logic are
> complete.

## Layout

```
main.go                     entrypoint + config
internal/
  model/      domain types (User, MoodEntry, Session, DayAverage)
  store/      Store interface + SQLite implementation + schema.sql
  service/    timezone "today", faces/colors, grid building, streak logic
  web/        server, middleware, routes, handlers, templates, static assets
```

## What's implemented (plan phases 1–3)

- **Auth**: signup/login/logout, bcrypt hashing, server-side sessions in a
  cookie (HttpOnly, SameSite=Lax).
- **Logging**: one entry per user per day (UPSERT), keyed to the user's local
  date. Timezone is captured at signup from the browser.
- **You grid**: ~52 weeks, colored cells, hover/tap tooltips.
- **Streaks**: current + longest. Current anchors on today if logged, otherwise
  yesterday, so an unlogged "today" doesn't prematurely break the streak.
- **Everyone**: per-day average across all users, with count in the tooltip.

## Notes & deviations from the plan

- **Handlers live in one `web` package** as methods on `*Server` (rather than a
  separate `handlers/` package) so they share the store and parsed templates
  without extra wiring.
- **Templates and static assets live under `internal/web/`** so they can be
  bundled with `//go:embed` — the binary is self-contained.
- **The schema lives at `internal/store/schema.sql`** (embedded and applied at
  startup) instead of a separate `migrations/` directory.
- **Timestamps are stored as RFC3339 TEXT** and dates as `YYYY-MM-DD` TEXT, to
  avoid driver-specific datetime quirks.
- **htmx is loaded from a CDN** in the layout for simplicity; vendor it locally
  if you want zero external requests.

## Still to do (plan phase 4 — polish/hardening)

- **CSRF tokens** on POST forms (currently relying on SameSite=Lax).
- Set cookie `Secure` unconditionally behind TLS in production (it's currently
  set only when the request itself is TLS).
- Smooth color interpolation on the Everyone grid (currently rounds to the
  nearest band for color; the exact average is shown in the tooltip).
- Tests for the streak logic and grid bounds.
- Postgres `Store` implementation for scale (the interface is already in place).
