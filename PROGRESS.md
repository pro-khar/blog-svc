# blog-svc — Progress Tracker

Learning project: CRUD blog service in Go + Postgres, for prakhar.space.
Approach: building step-by-step by hand, minimal AI help — AI is only used to
review/explain, not to write handler/business logic code.

## Stack
- Go (module `github.com/pro-khar/blog-svc`, go 1.25.7)
- chi router (`go-chi/chi/v5`)
- pgx/v5 (`jackc/pgx/v5`) — using `pgxpool`, no ORM
- Postgres 16 via docker-compose
- goose-style migration comments in `migrations/0001_init.sql` (goose itself not wired up yet — no CLI/tool invocation found in repo)
- godotenv for `.env` loading

## What exists and works
- `cmd/api/main.go` — loads env, opens pgx pool, pings it, builds router, serves on `:8080`. Done.
- `internal/db/db.go` — `NewPool(ctx, dsn)` returns a pinged `*pgxpool.Pool`. Done.
- `internal/model/post.go` — `Post` struct matches the `posts` table columns. Done. **Known gap**: `Excerpt` is a plain `string` but the `excerpt` column is nullable — a real `NULL` there will make `rows.Scan` error out. Not hit yet (test post has an excerpt), but flagged for whenever a post is created without one.
- `internal/http/router.go` — routes wired: `GET /health`, `GET /posts`, `GET /posts/{slug}`, `POST /posts`, `PUT /posts/{id}`, `DELETE /posts/{id}`. Middleware: RequestID, RealIP, Logger, Recoverer. Done.
- `migrations/0001_init.sql` — `posts` table (id, title, slug, excerpt, content_md, tags, created_at, updated_at, published_at, is_published). Applied and confirmed against the Docker Postgres container.
- `docker-compose.yml` — single postgres service, env-driven creds, named volume. Done, actively used (see "Local dev environment" below).
- **`internal/http/handlers.go` — `GetPosts` is now real** (done 2026-07-12). Runs:
  `SELECT id, title, slug, excerpt, tags, created_at, updated_at, is_published FROM posts WHERE is_published = true ORDER BY created_at DESC`
  Deliberately excludes `content_md` (list view shouldn't ship full body — use `excerpt`) and `published_at` (not needed by the list view yet; add back if the frontend needs it). Verified end-to-end with `curl localhost:8080/posts` returning real data after inserting a test row (`id 1`, slug `hello-world`).
- **`GetPostBySlug` is now real** (done 2026-07-12). Runs:
  `SELECT id, title, slug, excerpt, tags, created_at, updated_at, is_published FROM posts WHERE slug = $1 AND is_published = true`
  via `QueryRow` + `Scan`, with `errors.Is(err, pgx.ErrNoRows)` → 404, any other scan/query error → 500. On success, JSON-encodes the single `post`. Verified manually against both an existing slug (`hello-world`, returns 200 + post) and a nonexistent one (returns 404).

## What's stubbed (no DB access yet)
In `internal/http/handlers.go`:
- `CreatePost` — no body parsing, no insert, just `201`.
- `UpdatePost` — no body parsing, no update, just `200`.
- `DeletePost` — no id parsing, no delete, just `204`.

## Concepts learned building `GetPosts` (useful reference for the remaining handlers)
- **The standard Go DB-call skeleton**, reused for every query: `result, err := <call>` → `if err != nil { handle; return }` → use `result`. This applies to `Query`, `QueryRow`, and `Exec` alike.
- **Multi-row query shape**: `rows, err := h.DB.Query(ctx, sql, args...)` → check err → `defer rows.Close()` → `for rows.Next() { scan into a fresh struct each iteration; append }` → **after the loop**, check `rows.Err()` (catches errors that happen mid-stream, which a plain `false` from `Next()` can't distinguish from "ran out of rows normally").
- **`rows.Scan(&f1, &f2, ...)` is positional, not name-based** — argument order must exactly match the `SELECT` column order. Prefer explicit column lists over `SELECT *` so the two lists sit side by side and stay easy to eyeball against each other; `*` silently changes order/contents if the table schema changes later.
- **pgx v5 scans `TEXT[]` directly into Go `[]string`** and nullable timestamp columns directly into `*time.Time` — no manual conversion needed.
- **Don't scan a column you filtered on and then drop it from the struct** — e.g. filtering `WHERE is_published = true` but not scanning `is_published` back into the struct leaves the Go zero-value (`false`) in the JSON response, which actively lies about the data. Either scan it for real, or don't expose the field at all — don't leave it unscanned-but-present.
- **`var x []T` (nil slice) vs `x := []T{}` (empty non-nil slice)** — both behave identically in Go code (`append`, `len`, `range` all work the same), but `encoding/json` marshals nil as `null` and empty-non-nil as `[]`. Matters for frontend consumers (`null.map()` throws, `[].map()` doesn't) — decided on `[]T{}` here for that reason.
- **Row-filtering (`WHERE`) and column-selection (`SELECT` list) are independent concerns** — trimming which columns you select never leaks additional rows; only the `WHERE` clause controls which rows are reachable at all.
- **Context**: pass `r.Context()` (the request's context) into DB calls in handlers, not `context.Background()` — ties the query's lifetime to the HTTP request so it can be cancelled if the client disconnects.

## Concepts learned building `GetPostBySlug`
- **`Query` vs `QueryRow`**: `Query` returns `(pgx.Rows, error)` — the error only covers query-start failures; whether rows exist is discovered separately via `Next()`. `QueryRow` returns just `pgx.Row`, no error — the error only surfaces later, from `.Scan(...)`. `QueryRow` needs no `Close()`/loop since it's always zero-or-one rows.
- **Parameterized queries**: never string-concatenate user input into SQL (injection risk). Use positional placeholders (`$1`, `$2`, ... in Postgres) in the query string, and pass the actual values as trailing arguments to `Query`/`QueryRow`/`Exec` — the driver sends query and values separately so input is never parsed as SQL syntax.
- **`pgx.ErrNoRows`**: the sentinel error `Scan` returns when a `QueryRow` matched zero rows — this is the signal to return 404 instead of 500. Requires importing `github.com/jackc/pgx/v5` directly (separate from `pgxpool`).
- **`errors.Is(err, pgx.ErrNoRows)` over `err == pgx.ErrNoRows`**: both work when the error is unwrapped, but `errors.Is` also correctly unwraps errors that got wrapped by an intermediate layer (e.g. `fmt.Errorf("...: %w", err)`), so it's the safer default for any sentinel-error check.
- **`chi.URLParam(r, "name")`** pulls a `{name}` path segment out of the route pattern as a plain string — already free from the router, no manual parsing.

## Local dev environment (done, updated 2026-07-12)
This machine had **no Go and no Docker** installed at the start of this whole process —
worth knowing if setting up a new machine again.
- Initially set up **native Postgres 17** as a stopgap (Docker needed WSL2 + a reboot).
  Later, WSL2 + Docker Desktop got installed properly, so we **switched to the
  `docker-compose.yml` Postgres container** as the real local DB.
- Native Postgres 17 (Windows service `postgresql-x64-17`) is now **stopped** and set to
  `Manual` startup (not uninstalled — still there if ever needed, just not running/using
  port 5432 anymore).
- Docker Desktop for Windows + WSL2 (Ubuntu) is the container runtime. Gotcha hit:
  Docker Desktop's "Resource Saver mode" and per-distro WSL Integration toggle
  (Settings → Resources → WSL Integration) both need to be on/off correctly for
  `docker` commands run from inside the WSL Ubuntu shell to reach the daemon.
- Repo lives on the Windows filesystem; from WSL it's reached via
  `/mnt/c/Users/prakh/blog-svc`. Run `docker compose up -d` from there (in the Ubuntu
  terminal) to start the `blog-svc-postgres` container — reads `POSTGRES_USER` /
  `POSTGRES_PASSWORD` / `POSTGRES_DB` from `.env` in the repo root for compose variable
  substitution.
- Installed Go 1.26.5 via winget (`GoLang.Go`) — wasn't present at all before.
  Reminder: after any PATH-affecting install, already-open terminals (and apps like VS
  Code) keep their stale PATH — close and reopen the whole app/terminal, not just a new
  tab, to pick up the change.
- Installed the `goose` migration CLI via `go install github.com/pressly/goose/v3/cmd/goose@latest`
  (binary lands at `~/go/bin/goose.exe`; not on PATH by default in Windows shells).
- `migrations/0001_init.sql` applied and tracked by goose against the **container** DB
  (`goose_db_version` table shows `0001_init.sql` applied). Note: the container's
  Postgres data lives in the `pgdata` Docker volume — it's a fresh DB, separate from
  whatever was in the earlier native-Postgres `blog_svc` database.
- `.env` at repo root (gitignored) now has both the app's `DATABASE_URL` and the
  compose-file vars:
  ```
  DATABASE_URL=postgres://postgres:<password>@localhost:5432/blog_svc?sslmode=disable
  POSTGRES_USER=postgres
  POSTGRES_PASSWORD=<password>
  POSTGRES_DB=blog_svc
  ```
- Verified `go build ./...` succeeds and `go run ./cmd/api` logs "Database connected"
  against the Docker container — confirmed working end to end.

**To run locally**:
1. Make sure Docker Desktop is running (not in Resource Saver mode) and WSL integration
   for your distro is enabled.
2. From WSL: `cd /mnt/c/Users/prakh/blog-svc && docker compose up -d` (or run
   `docker compose up -d` from a Windows shell if `docker` is on PATH there instead).
3. From a Windows shell (PowerShell/VS Code terminal) in the repo root: `go run ./cmd/api`.
   Server listens on `:8080`.

## Test data
One row exists in the `blog_svc` DB (inside the Docker container, `pgdata` volume) as of
2026-07-12: `id 1`, title "Hello World", slug `hello-world`, `is_published = true`. Useful
for testing `GetPostBySlug` next — query for `hello-world` should return this row; query
for any other slug should hit the not-found path.

## Next steps (in order)
1. ~~Confirm DB is actually reachable and migrated~~ — **done**, see above.
2. ~~`GetPosts` — first real query~~ — **done**, see "What exists and works" and the
   concepts list above.
3. ~~`GetPostBySlug` — parameterized query + not-found handling~~ — **done**, see concepts
   list above.
4. **`CreatePost` — decode + insert** ← **pick up here next**
   - Decode JSON body into a `Post` (or a separate "create" DTO — worth deciding: same struct vs slim input struct without id/timestamps).
   - Learn: request body validation (required fields, slug format/uniqueness), `INSERT ... RETURNING id, created_at, updated_at`, mapping unique-constraint violation (duplicate slug) to 409.
5. **`UpdatePost` — partial vs full update**
   - Decide: PUT = full replace (simpler, matches REST semantics) vs PATCH-style partial. Given route is `PUT`, plan a full-replace update statement keyed by id.
   - Learn: `chi.URLParam` for `{id}`, converting string→int safely, `UPDATE ... SET ... WHERE id = $n`, checking `RowsAffected()` to detect "not found".
6. **`DeletePost`**
   - `DELETE FROM posts WHERE id = $1`, check `RowsAffected()` for 404 vs 204.
7. **Cross-cutting, after basic CRUD works**
   - Consistent error response shape (JSON error body vs empty body).
   - Input validation layer (empty title, slug format).
   - Context timeouts per request (`r.Context()` passed into DB calls instead of `context.Background()`).
   - Basic tests (handler tests with a test DB or sqlmock-style, or integration test against docker-compose postgres).
   - Pagination for `GET /posts` once there's more than a couple rows.
   - Auth for write endpoints (currently anyone can POST/PUT/DELETE) — needed before this is internet-facing.

## How to resume this in a new session
Point Claude at this file and say "read PROGRESS.md and tell me what's next." It should re-derive the same picture: stubs still in `handlers.go`, next unchecked item in the list above is where to pick up. Update the "What's stubbed" / "Next steps" sections as items get done — move completed items into "What exists and works" with a one-line note.
