# logmood

A minimalist mood tracker at [logmood.com](https://logmood.com). Log one mood a
day (five faces, 🙁 → 😄), add an optional note, see your year as a
GitHub-style activity grid with streaks and weekly insights, and view an
**everyone** tab showing the daily average across all users. Individual entries
stay private; the community view only exposes aggregates.

Server-rendered Go (no SPA, no build step). Stack: **chi** router, **PostgreSQL**
database, **bcrypt** password hashing, `html/template` + **htmx** for
interactivity. Light/dark theme using Catppuccin (Latte/Mocha).

## Development

Requires Go 1.22+ and a PostgreSQL instance.

```bash
go mod tidy
DB_DSN="postgres://mood:mood@localhost:5432/mood?sslmode=disable" go run .
```

Then open http://localhost:8080 — sign up and start logging.

### Environment variables

| variable | default | description |
|---|---|---|
| `ADDR` | `:8080` | listen address |
| `DB_DSN` | `postgres://mood:mood@localhost:5432/mood?sslmode=disable` | PostgreSQL connection string |

### Make targets

| target | description |
|---|---|
| `make run` | run the server locally |
| `make build` | compile binary to `./mood-tracker` |
| `make test` | run the test suite |
| `make seed` | seed local db with 100 test accounts |
| `make seed-docker` | seed the running docker compose db |
| `make up` | start docker compose in the background |
| `make down` | stop docker compose |
| `make logs` | tail app container logs |
| `make clean` | remove compiled binary |
| `make deploy HOST=<ip>` | deploy to the droplet |

## Database migrations

Migrations are numbered SQL files in `internal/store/migrations/` and are applied
automatically on startup. The app tracks which migrations have run in a
`schema_migrations` table — already-applied versions are skipped.

```
internal/store/migrations/
  001_initial.sql          # tables: users, sessions, mood_entries
  002_add_username.sql     # adds username column to users
```

### Adding a new migration

1. Create a new file following the naming convention: `NNN_description.sql`
   (e.g. `003_add_avatar.sql`).
2. Write idempotent SQL — use `IF NOT EXISTS` / `IF EXISTS` where possible so
   re-runs are safe.
3. Deploy or restart the app — migrations run automatically on startup.

### How it works

On startup, `OpenPostgres` creates the `schema_migrations` table (via
`schema.sql`), then reads all `*.sql` files from the embedded `migrations/`
directory, sorts them by version number, and runs any that haven't been applied
yet. Each successful migration is recorded in `schema_migrations` with a
timestamp.

No external migration tool is required — the app is self-migrating.

## Deployment

The app runs on a DigitalOcean droplet behind Cloudflare (DNS proxy + origin
certificates for HTTPS). Docker Compose manages three services:

- **postgres** — PostgreSQL 17 (Alpine), data persisted in a named volume
- **app** — the Go binary (built from scratch), connects to postgres on port 8080
- **caddy** — reverse proxy, terminates TLS with Cloudflare origin certificates,
  redirects HTTP → HTTPS

### TLS certificates

Cloudflare origin certificates are stored on the droplet at `~/certs/`:

```
~/certs/origin.pem        # certificate
~/certs/origin-key.pem    # private key
```

These are mounted into the Caddy container (read-only). The Caddyfile serves
HTTPS on :443 with the origin cert and redirects :80 → HTTPS.

### CI/CD

Pushing to `main` triggers the GitHub Actions workflow (`.github/workflows/deploy.yml`):

1. Builds the Docker image and pushes to `ghcr.io/alvinnguyen0/mood:latest`
2. SSHes into the droplet and runs `scripts/deploy.sh` (pulls latest image,
   restarts containers)

Migrations run automatically when the new container starts.

## Seed test data

`cmd/seed` populates the database with test accounts and ~a year of varied mood
history, so the **everyone** grid and individual **you** grids have data to
render.

```bash
go run ./cmd/seed                  # 100 accounts, ~365 days of history
go run ./cmd/seed -n 50 -seed 7    # 50 accounts, reproducible RNG
go run ./cmd/seed -docker          # seed the running docker compose db
```

Accounts are `user001@example.com` … `userNNN@example.com`, password
`password123` (override with `-password`). Re-running is idempotent.

## Layout

```
main.go                         entrypoint
Dockerfile                      multi-stage build (scratch final image)
docker-compose.yml              postgres + app + caddy
Caddyfile                       TLS termination + reverse proxy
scripts/deploy.sh               droplet deploy script
internal/
  model/                        domain types (User, MoodEntry, Session, DayAverage)
  store/                        Store interface + Postgres implementation
    migrations/                 numbered SQL migration files
  service/                      grid building, streaks, trends, colors
  web/                          server, middleware, routes, handlers
    templates/                  html/template pages + partials
    static/                     CSS + JS (embedded via go:embed)
```

## Features

- **auth** — signup/login/logout, bcrypt hashing, server-side sessions
  (HttpOnly, SameSite=Lax cookie)
- **mood logging** — one entry per user per day (upsert), optional note (50
  chars), keyed to user's local date
- **you page** — activity grid, current/longest streaks, weekly average with
  delta, most common mood, day-of-week breakdown
- **everyone page** — community average grid, today's log count
- **my account** — view email, set/change username, change password
- **theme** — light (Catppuccin Latte) / dark (Catppuccin Mocha) toggle,
  persisted in localStorage
- **responsive** — works on mobile; grid scrolls horizontally and snaps to
  most recent entries
