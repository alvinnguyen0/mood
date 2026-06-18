# Mood Tracker — Project Plan

A minimalistic web app for logging a daily mood and viewing it as a
GitHub-style activity grid, plus an aggregate "everyone" view of the
collective mood. Go backend, server-rendered frontend, account-based auth.

---

## 1. Concept & Design Principles

- **One action per day:** pick today's mood from five faces. That's the core loop.
- **The grid is the centerpiece.** Rows = days of week, columns = weeks, one
  colored cell per day — mirroring GitHub's contribution graph.
- **Minimal and calm:** generous whitespace, a restrained palette, no chrome
  beyond what the data needs.
- **Responsive:** the grid scrolls horizontally on narrow screens; everything
  else stacks cleanly. Touch and hover both reveal cell details.

### Mood scale

| Level | Face          | Meaning      | Cell color (suggested) |
|-------|---------------|--------------|------------------------|
| 1     | 🙁 frown      | worst        | `#5b7fa6` muted slate-blue (cool) |
| 2     | 😕 meh        | low          | `#6fa8a0` desaturated teal |
| 3     | 😐 neutral    | middle       | `#e6c35c` soft amber |
| 4     | 🙂 slight smile | good       | `#f0954e` warm orange |
| 5     | 😄 big smile  | best         | `#f06d4e` bright coral (warm) |
| —     | (none)        | not logged   | `#ebedf0` neutral gray |

The scale runs cool→warm, low→high. The gray for unlogged days matches
GitHub's empty-cell convention so the "fill the grid" instinct carries over.
The `mood_level` integer is the source of truth; colors are a presentation
concern and live only in the frontend (CSS variables / a JS lookup map).

---

## 2. Tech Stack & Rationale

### Backend: Go + chi router

- **`net/http` with [chi](https://github.com/go-chi/chi)** as a thin router.
  chi is idiomatic, dependency-light, and gives clean URL params and
  composable middleware (auth, logging, recovery) without adopting a heavy
  framework. Standard `net/http` handlers throughout — chi just routes.

### Database: SQLite (MVP) → Postgres (scale path)

- **SQLite** for the first build. It's zero-config, ships as a single file,
  deploys with the binary, and — in WAL mode — comfortably handles the
  concurrency a small multi-user app generates. It fits the minimalist ethos:
  no separate DB server to run.
- The relational model is a perfect fit: two tables, a uniqueness constraint,
  and the aggregate query (`AVG`, `GROUP BY`) the Everyone view needs are
  trivial in SQL.
- **Migration path:** keep all SQL ANSI-ish and route DB access through a small
  repository layer so swapping to **Postgres** later (for higher write
  concurrency or hosted deployment) is a driver change, not a rewrite.
- Driver: `modernc.org/sqlite` (pure-Go, no cgo) for easy cross-compilation.

### Frontend: server-rendered Go templates + htmx

- **`html/template`** renders pages and grid partials server-side. No build
  step, no bundler, no separate frontend toolchain — which keeps the whole app
  a single Go binary plus static assets.
- **[htmx](https://htmx.org)** handles the only real interactivity (logging a
  mood, swapping the picker for the "today's mood" state, switching tabs)
  by swapping HTML fragments returned from the server. This avoids a full SPA
  while still feeling instant.
- A small amount of **vanilla JS + CSS** for tooltips on grid cells.
- *Why not a SPA?* The app is read-heavy and structurally simple. A React/Vue
  SPA would add a build pipeline, a JSON serialization layer, and client state
  for very little benefit. Server rendering keeps the surface area small and
  the payload tiny.

### Authentication: server-side sessions + cookies

- **Session cookies** over JWT. For a server-rendered app, sessions are simpler:
  the cookie holds an opaque session ID, server state lives in a `sessions`
  table (or an in-memory store backed by the DB), and logout/revocation is a
  single delete. No token-refresh dance, no client-side token storage.
- Cookies are `HttpOnly`, `Secure`, `SameSite=Lax`.
- Passwords hashed with **bcrypt** (cost ~12) or **argon2id**. Never stored or
  logged in plaintext.

---

## 3. Architecture

```
Browser (htmx + minimal JS/CSS)
        │  HTML over HTTP (page loads + fragment swaps)
        ▼
Go HTTP server (chi)
  ├── middleware:  recover · logging · session-auth
  ├── handlers:    auth · mood · personal-grid · everyone-grid
  ├── services:    streaks · aggregation · timezone/today
  ├── repository:  users · mood_entries · sessions   (interface)
  └── templates:   layout · you · everyone · partials (grid, picker)
        │
        ▼
SQLite (WAL)  →  Postgres (later)
```

Layering: handlers do HTTP + template rendering; services hold the logic
(streak computation, averaging, "what is today for this user"); the repository
interface isolates SQL so the DB is swappable and the logic is unit-testable.

---

## 4. Data Model

Two core tables plus a sessions table.

### `users`
| column          | type        | notes                                  |
|-----------------|-------------|----------------------------------------|
| `id`            | INTEGER PK  |                                        |
| `email`         | TEXT UNIQUE | login identifier (or `username`)       |
| `password_hash` | TEXT        | bcrypt/argon2id                        |
| `timezone`      | TEXT        | IANA name, e.g. `America/New_York`; defaults to UTC, set at signup |
| `created_at`    | TIMESTAMP   |                                        |

### `mood_entries`
| column        | type       | notes                                    |
|---------------|------------|------------------------------------------|
| `id`          | INTEGER PK |                                          |
| `user_id`     | INTEGER FK | → `users.id`                             |
| `entry_date`  | DATE       | the user's **local** calendar date       |
| `mood_level`  | INTEGER    | 1–5, CHECK constraint                     |
| `created_at`  | TIMESTAMP  |                                          |
| `updated_at`  | TIMESTAMP  |                                          |

**One entry per user per day** is enforced by a DB constraint:
`UNIQUE (user_id, entry_date)`. Logging uses an UPSERT
(`INSERT ... ON CONFLICT (user_id, entry_date) DO UPDATE SET mood_level=..., updated_at=...`),
so re-logging the same day edits the existing row instead of creating a second.

### `sessions`
| column       | type       | notes                          |
|--------------|------------|--------------------------------|
| `id`         | TEXT PK    | random opaque token (in cookie)|
| `user_id`    | INTEGER FK |                                |
| `expires_at` | TIMESTAMP  |                                |

Indexes: `mood_entries (user_id, entry_date)` for the personal grid;
`mood_entries (entry_date)` for the Everyone aggregate.

---

## 5. Key Logic

### "Today" & timezone handling

"Today" is **per user**, derived from the user's stored IANA `timezone`. On
every request that needs it, compute `today = now().In(userTZ)` and take the
calendar date. Mood logging writes that local date into `entry_date`. Two
users in different zones logging at the same instant may write different
`entry_date` values — that's intended; each person's day is their own.

The Everyone aggregate groups by `entry_date` as stored (each user's local
date). This keeps the query trivial; the small cross-timezone smearing at day
boundaries is an acceptable simplification for a mood app and is noted rather
than engineered away.

### Current & longest streak

Fetch the user's `entry_date` values (logged days are a set; mood level doesn't
matter for streaks — only presence does).

- **Longest streak:** walk the sorted dates, counting the longest run where
  each date is exactly one day after the previous; track the max.
- **Current streak:** count consecutive logged days walking *backwards* from an
  anchor:
  - If **today is logged**, anchor = today.
  - If **today is not yet logged**, anchor = **yesterday** — so an unlogged
    "today" does not prematurely break the streak (the user simply hasn't
    logged yet today). The current streak counts back from yesterday.
  - If neither today nor yesterday is logged, current streak = **0** (the gap
    is real).

This mirrors GitHub: your streak survives "today, not done yet" but breaks the
moment a full day passes with no entry. Computed in Go from the date set so the
rule is explicit and testable.

### Everyone aggregate (averages)

```sql
SELECT entry_date,
       AVG(mood_level) AS avg_mood,
       COUNT(*)        AS n
FROM mood_entries
GROUP BY entry_date;
```

`avg_mood` is a float in `[1.0, 5.0]`. The frontend maps it to a color by
interpolating between the band colors (or rounding to the nearest level for a
simpler banded look). The tooltip shows the date, the average (e.g. "3.7
avg"), and `n` (how many people logged that day). Days with no entries from
anyone render as the empty-gray cell.

---

## 6. HTTP API Surface

Server-rendered routes return full pages or htmx fragments; data routes are
listed with the payload they carry. (If a JSON SPA is ever chosen, the same
surface returns JSON instead of HTML.)

### Auth
| Method & path        | Purpose                                   |
|----------------------|-------------------------------------------|
| `GET  /signup`       | signup form                               |
| `POST /signup`       | create account, set session, redirect     |
| `GET  /login`        | login form                                |
| `POST /login`        | verify password, set session cookie       |
| `POST /logout`       | delete session, clear cookie              |

### Mood logging
| Method & path        | Purpose                                            |
|----------------------|----------------------------------------------------|
| `POST /mood`         | upsert today's mood; body `mood_level=1..5`. Returns the updated "today" partial. |

### Personal view ("You")
| Method & path        | Purpose                                            |
|----------------------|----------------------------------------------------|
| `GET  /` or `/you`   | personal page: today's picker/status, grid, streaks|
| `GET  /you/grid`     | grid + streak data (fragment) — date→mood_level map, current & longest streak, today's entry |

### Community view ("Everyone")
| Method & path          | Purpose                                          |
|------------------------|--------------------------------------------------|
| `GET  /everyone`       | aggregate page                                   |
| `GET  /everyone/grid`  | aggregate fragment — per date: avg_mood + count  |

All non-auth routes sit behind session-auth middleware; unauthenticated
requests redirect to `/login`.

---

## 7. UI Components

- **Nav bar** — two tabs only: **You** | **Everyone**. Active tab highlighted.
- **Mood picker** — the five faces as large tappable targets; the core control.
- **Today's status card** — when today is already logged, shows the chosen face
  and an "edit" affordance that swaps back to the picker.
- **Mood grid** — the reusable centerpiece: weekday rows × week columns of
  colored cells, with month labels along the top and weekday labels down the
  side. Used by both tabs (personal mood vs. aggregate average).
- **Grid cell + tooltip** — on hover/tap shows: *You* → date + mood; *Everyone*
  → date + average mood + number of people who logged.
- **Streak display** — "Current: N days · Longest: M days," shown above or
  beside the personal grid.
- **Color legend** — small "less ←→ more"/low→high key, GitHub-style.
- **Auth pages** — minimal login and signup forms.
- **Empty states** — gentle prompt to log your first mood when the grid is bare.

---

## 8. Considerations Checklist

- **One entry per user per day:** DB `UNIQUE(user_id, entry_date)` + UPSERT.
- **Editing today:** same UPSERT path; re-picking updates `mood_level` and
  `updated_at`.
- **Timezone:** store IANA tz per user; derive local "today" on every relevant
  request; write local `entry_date`.
- **Privacy:** personal entries are scoped by `user_id` and never exposed
  individually; the Everyone view only ever returns aggregates (avg + count).
- **Aggregate with few users:** counts shown so a day with `n=1` reads honestly
  rather than implying consensus.
- **Security:** hashed passwords, `HttpOnly`/`Secure`/`SameSite` cookies, CSRF
  protection on state-changing POSTs, parameterized SQL.

---

## 9. Phased Build Order

### Phase 0 — Scaffolding
Project layout, chi server, SQLite connection (WAL), migrations for `users`,
`mood_entries`, `sessions`, base layout template, static asset serving.

### Phase 1 — MVP: auth + log + personal grid
- Signup / login / logout with sessions and hashed passwords.
- Mood picker → `POST /mood` UPSERT for today (with timezone-aware date).
- "Today's mood" status with edit.
- Personal grid rendering logged days in mood colors.
- **Milestone:** a user can sign up, log a mood daily, and see it on their grid.

### Phase 2 — Streaks & cell detail
- Current + longest streak computation (with the today/yesterday rule).
- Streak display.
- Cell tooltips (date + mood) and the color legend.

### Phase 3 — Everyone aggregate view
- Aggregate query (avg + count per date).
- Everyone grid + tab + tooltips (date, average, people count).

### Phase 4 — Polish ✅
- Responsive/mobile pass: horizontal-scroll grid, stacked layout, touch
  tooltips.
- Color/whitespace refinement, empty states.
- Harden timezone edges, CSRF, session expiry, input validation.
- **Stretch:** Postgres adapter, basic tests for streak/aggregate logic.

### Phase 5 — Social proof & engagement
- **"X people logged today" counter** on the Everyone page. Live count of
  today's entries creates social proof and FOMO. Small query
  (`COUNT(*) WHERE entry_date = today`), shown as a subtle line above or
  below the everyone grid.
- **One-word note per entry.** Add a nullable `note` column (max ~50 chars) to
  `mood_entries`. Shown in the mood picker as an optional text field, and in
  cell tooltips on the You page. Keeps logging fast but adds texture for
  looking back at *why* a day felt a certain way.

### Phase 6 — You page trends & insights
- **Weekly average with delta** — "your average this week: 3.8 (+0.4 vs last
  week)" displayed near the streaks. Gives a reason to check back beyond
  just logging.
- **Most common mood** — which face appears most often across all entries.
- **Day-of-week breakdown** — show which weekdays tend to be best/worst
  (bar chart or simple list: "your best day is Saturday, worst is Monday").
- **Monthly averages** — small trend line or list of monthly averages so
  users can spot seasonal patterns.
- **Time-of-day insight** (requires `logged_at` timestamp on entries) —
  "you tend to feel better when you log in the morning."

### Phase 7 — Shareable mood recap
- **Year-in-review / mood recap page** — a single shareable page (or
  screenshot-friendly layout) showing the user's grid, average mood,
  longest streak, most common mood, and monthly trend. Accessed via a
  unique share link (`/share/:token`).
- Add `share_token` (TEXT UNIQUE, nullable) to `users`. Generated on
  demand when the user first requests their share link.
- The share page is public (no auth) but read-only and contains no PII
  beyond what the user chose to share.
- **This is the primary viral mechanic:** users screenshot or link their
  recap, others see it and sign up.

---

## 10. Future Schema Additions

| Table | Column | Type | Phase | Enables |
|-------|--------|------|-------|---------|
| `mood_entries` | `note` | TEXT (nullable, max ~50 chars) | 5 | one-word/short note per day |
| `mood_entries` | `logged_at` | TIMESTAMP | 6 | time-of-day analysis |
| `users` | `display_name` | TEXT (nullable) | 7 | sharing features |
| `users` | `share_token` | TEXT UNIQUE (nullable) | 7 | public recap link |
| `users` | `signup_source` | TEXT (nullable) | — | growth analytics |

---

## 11. Features intentionally excluded

These add complexity that breaks the "minimal and calm" ethos:
- Comments / social feed / reactions to others' moods
- Friend lists or following
- Push notifications
- Gamification badges
- Long-form mood journaling
- Leaderboards

---

## 12. Suggested Project Layout

```
mood-tracker/
├── main.go
├── go.mod
├── internal/
│   ├── server/        # chi routes + middleware
│   ├── handlers/      # auth, mood, you, everyone
│   ├── service/       # streaks, aggregate, today/tz
│   ├── store/         # repository interface + sqlite impl
│   └── model/         # User, MoodEntry, Session
├── templates/         # layout, you, everyone, partials
├── static/            # css, htmx, minimal js
└── migrations/        # schema SQL
```
