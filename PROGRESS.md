# blog-svc — Progress Tracker

Learning project: CRUD blog service in Go + Postgres, for prakhar.space.
Approach: building step-by-step by hand, minimal AI help — AI is only used to
review/explain, not to write handler/business logic code.

## Stack
- Go (module `github.com/pro-khar/blog-svc`, go 1.25.7)
- chi router (`go-chi/chi/v5`)
- pgx/v5 (`jackc/pgx/v5`) — using `pgxpool`, no ORM
- Postgres 16 via docker-compose
- goose for migrations (`migrations/0001_init.sql`, applied via the goose CLI — see "Local dev environment")
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
- **`CreatePost` passes 1 and 2 are real.**
  - **Pass 1, happy path** (done 2026-10-02, commit `d5f30d4`). Decodes the JSON body straight into a `model.Post`
    (bad JSON → 400), then runs
    `INSERT INTO posts (title, slug, excerpt, content_md, tags, is_published) VALUES ($1..$6) RETURNING id, created_at, updated_at`
    via `QueryRow`, scans the returned values back into the same `post`, and responds `201` + the post as JSON.
    Multi-line SQL is written as a Go raw string (backticks). Verified: posts created through the API are in the DB (ids 2, 3, 5).
  - **Pass 2, validation** (done 2026-10-03). Runs after decoding and before the insert. Each failure responds
    **422 Unprocessable Entity** with a plain-text message and returns:
    - slug empty or whitespace-only (`strings.TrimSpace`) → "Slug cannot be empty"
    - slug contains whitespace or uppercase letters (two `regexp` checks) → "Malformed Slug, please watch for spaces and capitals"
    - title empty or whitespace-only → "Title cannot be empty"
    - content empty or whitespace-only → "Content cannot be empty"

    Design choice made here: **400 for JSON that can't be parsed, 422 for JSON that parses but has invalid values.**
  - **Pass 3 (duplicate slug → 409) is still to do** — see "Known gaps in `CreatePost`" below.

## What's stubbed (no DB access yet)
In `internal/http/handlers.go`:
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

## Concepts learned building `CreatePost`
This was the first handler that takes input in the body, and it needed a mental-model shift from frontend thinking:
- **A handler is the other end of a `fetch` call.** `r` holds everything the frontend put into its `fetch` (URL params → `chi.URLParam`,
  query string → `r.URL.Query().Get`, body → `r.Body`). `w` is the blank response you fill in for the frontend to read
  (`res.status` ← `w.WriteHeader`, `await res.json()` ← `json.NewEncoder(w).Encode`).
- **Useful technique: write the `fetch` first.** Before writing a handler, write (or picture) the frontend `fetch` that would hit it
  and what the frontend does with each status code. Then the handler answers: where does each thing the fetch *sends* show up
  in `r`, and where do I write each thing the frontend *reads* to `w`. `temp.mts` in the repo root (untracked) is that fetch
  for `CreatePost` — runnable with `node temp.mts` to test the endpoint.
- **Every handler has the same four steps**: parse the input → validate it → do the DB work → respond.
  `GetPosts` has no input; `GetPostBySlug` parses a URL param; `CreatePost` is the first that needs all four.
- **The backend sets the contract, and input is untrusted.** Frontend validation is UX; backend validation is the real guard,
  because requests can come from curl/Postman/anyone, not just your form.
- **Why a struct is needed to decode JSON**: JS `JSON.parse` gives a shapeless object; Go needs a container with named slots
  declared up front. `json.NewDecoder(r.Body).Decode(&post)` is Go's `JSON.parse` (the mirror of `Encode`, which is `JSON.stringify`).
  The `&` is for the same reason as in `Scan` — the decoder writes into your variable.
- **Missing JSON keys leave fields at their zero value** (`""`, `false`, `nil`) — like destructuring defaults in JS. So a body
  without `is_published` creates a draft.
- **Struct fields must start with an uppercase letter** (exported) or `encoding/json` silently ignores them — no error, the
  field just stays empty. The JSON key can still be lowercase via the `json:"..."` tag.
- **One field, three names**: JSON key `content` ↔ Go field `ContentMD` (bridged by the `json` tag) ↔ column `content_md`
  (bridged by your SQL column list).
- **Bad JSON is the client's fault → 400**, not 500. 5xx means the server broke; 4xx means the request was wrong.
- **`INSERT ... RETURNING col1, col2`** makes Postgres send DB-generated values (id, default timestamps) back as a row, so
  you read them with `QueryRow` + `Scan`, exactly like a `SELECT` of one row.
- **SERIAL ids can have gaps** — a failed insert still uses up a sequence number (that's why there's no `id 4` in the test data).
- **Validation goes between decode and the DB call** (step 2 of parse → validate → work → respond), and each failed check
  responds and `return`s immediately, so invalid data never reaches the INSERT.
- **400 vs 422**: 400 Bad Request when the body can't be parsed at all; 422 Unprocessable Entity when it parses fine but the
  values break your rules. Both are 4xx because the client sent something wrong.
- **`strings.TrimSpace(x) == ""`** catches both empty and whitespace-only strings, which a plain `x == ""` misses.

## Known gaps in `CreatePost` (pick up here)
Found by looking at the rows created through the API:
1. ~~**No validation (pass 2).**~~ **Done 2026-10-03** — empty title/slug/content and slugs with spaces/capitals now get 422.
   Two follow-ups left from review (not blocking):
   - **The slug check is a blocklist** (rejects spaces and capitals). Other URL-unsafe characters still get through,
     e.g. `/`, `?`, `#`, `%`, `_`, accented letters, emoji, or leading/trailing/double hyphens. An **allowlist** check
     ("the whole slug must consist only of lowercase letters, digits and single hyphens") covers all of those in one rule.
     General rule: for input validation, describe what's allowed, not what's forbidden.
   - **The regexes are compiled on every request** (`regexp.MustCompile` inside the handler). Compiling once at package
     level (a `var` outside the function) is the idiomatic Go approach and avoids redoing the work per request.
2. **Duplicate slug returns 500 (pass 3).** ← **next** The `slug` column is `UNIQUE`, so a duplicate fails the insert with a generic
   server error. Should be 409 "slug already taken". (The missing `id 4` is very likely one such failed attempt.) Telling this
   error apart means inspecting Postgres's error code — the unique-violation code is `23505`, and pgx exposes it via `*pgconn.PgError`
   (`errors.As`, the type-matching sibling of `errors.Is`).
3. **`published_at` is never set.** Row `id 5` has `is_published = true` but `published_at` is NULL. Decide: set it to `now()`
   when a post is created as published (in SQL or in Go).
4. **Not yet tested: a body with no `tags` key.** `post.Tags` would be a nil slice; pgx most likely sends a nil slice as NULL,
   and the `tags` column is `NOT NULL` (its `DEFAULT '{}'` only applies when the column is left out of the INSERT entirely, not
   when NULL is passed explicitly). If that's right, creating a post without tags returns 500. Test it, then decide how to handle it.
5. **Decoding straight into `model.Post` lets the client send fields it shouldn't** (`id`, `created_at`, `published_at`, ...).
   The INSERT ignores them and `Scan` overwrites `id`/timestamps, but a client-sent `published_at` would be echoed back in the
   201 response without being stored. Fine for now; the clean fix is a separate, smaller input struct with only the fields a
   client may send (decided against for pass 1 to keep things simple).

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

**Port 8080 gotcha**: qBittorrent's Web UI also listens on 8080 by default. If `go run` logs
"Database connected" and then `bind: Only one usage of each socket address...`, check what owns
the port (`Get-NetTCPConnection -LocalPort 8080 -State Listen`) — last time it was qBittorrent;
quitting it fixed it.

## Test data
Rows in the `blog_svc` DB (inside the Docker container, `pgdata` volume) as of 2026-10-02:
- `id 1` — "Hello World", slug `hello-world`, published (inserted by hand via psql).
- `id 2` — slug `from-the-client-side`, draft (created through `POST /posts`).
- `id 3` — slug `Second post from-the-client-side`, draft — **invalid slug** (spaces/uppercase), kept as an example for validation.
- `id 5` — slug `Second-post-from-the-client-side`, published but `published_at` is NULL.
No `id 4` — a failed insert consumed that sequence value.

## Next steps (in order)
1. ~~Confirm DB is actually reachable and migrated~~ — **done**, see above.
2. ~~`GetPosts` — first real query~~ — **done**, see "What exists and works" and the
   concepts list above.
3. ~~`GetPostBySlug` — parameterized query + not-found handling~~ — **done**, see concepts
   list above.
4. **`CreatePost`** — pass 1 (decode + insert + 201) and pass 2 (validation → 422) **done**.
   ← **pick up here next** with the "Known gaps in `CreatePost`" list above: pass 3 (duplicate slug → 409),
   then `published_at`, the missing-`tags` case, and the slug-allowlist follow-up.
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
   - Read the listen port from a `PORT` env var (default `8080`) instead of hardcoding `:8080` in `cmd/api/main.go`.

## How to resume this in a new session
Point Claude at this file and say "read PROGRESS.md and tell me what's next." It should re-derive the same picture: stubs still in `handlers.go`, next unchecked item in the list above is where to pick up. Update the "What's stubbed" / "Next steps" sections as items get done — move completed items into "What exists and works" with a one-line note.
