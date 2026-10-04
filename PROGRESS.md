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

## API routes (as of 2026-10-04)
Split into **public** routes (what blog readers hit — published posts only) and **admin** routes under `/admin`
(writing posts and seeing drafts). Note: `/admin` is only a path prefix right now — **there is no auth yet**.

| Method | Path | Handler | State |
|---|---|---|---|
| GET | `/health` | inline in router | done |
| GET | `/posts` | `GetPosts` | done — published only, list shape (`model.List`, no content) |
| GET | `/posts/{slug}` | `GetPostBySlug` | done — published only. **Bug: doesn't return `content`** (see Open issues) |
| POST | `/admin/posts` | `CreatePost` | done — always creates a **draft** |
| GET | `/admin/posts` | `GetPostsInternal` | done — all posts including drafts, list shape |
| PUT | `/admin/posts/{id}` | `UpdatePost` | done — full replace of content fields |
| POST | `/admin/posts/{id}/publish` | `PublishPost` | **stub** — returns 501 Not Implemented |
| POST | `/admin/posts/{id}/un-publish` | `UnPublishPost` | **stub** — returns 501 Not Implemented |
| DELETE | `/admin/posts/{id}` | `DeletePost` | done — hard delete, any publish state |

Middleware on all routes: RequestID, RealIP, Logger, Recoverer.

## What exists and works
- `cmd/api/main.go` — loads env, opens pgx pool, pings it, builds router, serves on `:8080`. Done.
- `internal/db/db.go` — `NewPool(ctx, dsn)` returns a pinged `*pgxpool.Pool`. Done.
- `internal/model/post.go` — `Post` struct, matches all `posts` columns. Used for single posts and request bodies.
- `internal/model/list.go` — `List` struct (added 2026-10-04): the list-item shape — same as `Post` minus `ContentMD`.
  Used by `GetPosts` and `GetPostsInternal` so list responses never carry full post bodies.
- `migrations/0001_init.sql` — `posts` table (id, title, slug, excerpt, content_md, tags, created_at, updated_at, published_at, is_published). Applied and confirmed against the Docker Postgres container.
- `docker-compose.yml` — single postgres service, env-driven creds, named volume. Done, actively used (see "Local dev environment" below).
- **`GetPosts`** (done 2026-07-12, reshaped 2026-10-04). Runs
  `SELECT id, title, slug, excerpt, tags, created_at, updated_at, is_published, published_at FROM posts WHERE is_published = true ORDER BY created_at DESC`
  and scans each row into a `model.List`. `content_md` is deliberately left out — list views use `excerpt`.
- **`GetPostsInternal`** (added 2026-10-04). Same query and loop as `GetPosts` but without the `WHERE` filter, so the admin
  side sees drafts too.
- **`GetPostBySlug`** (done 2026-07-12). Runs
  `SELECT id, title, slug, excerpt, tags, created_at, updated_at, is_published FROM posts WHERE slug = $1 AND is_published = true`
  via `QueryRow` + `Scan`, with `errors.Is(err, pgx.ErrNoRows)` → 404, any other error → 500, success → 200 + post.
  Verified with an existing slug (200) and a nonexistent one (404). **Missing `content_md`** — see Open issues #1.
- **`CreatePost`** — built in three passes plus a fix:
  - **Pass 1, happy path** (2026-10-02, commit `d5f30d4`). Decodes the JSON body into a `model.Post` (bad JSON → 400),
    INSERTs it with `RETURNING id, created_at, updated_at`, scans those back into the same `post`, responds `201` + post.
    Multi-line SQL is written as a Go raw string (backticks).
  - **Pass 2, validation** (2026-10-03, commit `e875b79`). After decoding, before the INSERT. Each failure → **422** + plain-text message:
    slug empty/whitespace-only, slug containing whitespace or uppercase (two `regexp` checks), title empty, content empty.
    Design choice: **400 for JSON that can't be parsed, 422 for JSON that parses but has invalid values.**
  - **Pass 3, duplicate slug → 409** (2026-10-03, commit `a6a81b8`). `errors.As(err, &pgErr)` with `var pgErr *pgconn.PgError`;
    `pgErr.Code == "23505"` (unique violation) → **409** "Slug already exists". Any other error → 500.
  - **Missing `tags` fix** (2026-10-03, commit `a6a81b8`). `if post.Tags == nil { post.Tags = []string{} }` before the INSERT, so a
    body without `tags` stores `{}` instead of failing on NULL. Verified with rows `id 10` and `id 14`.
  - **Changed 2026-10-04: no longer accepts `is_published`.** The INSERT now only sets
    `title, slug, excerpt, content_md, tags`, so every new post is a draft (column default `false`). Publishing is
    moving to the dedicated publish endpoint.
- **`UpdatePost`** (done 2026-10-04, commit `51729a7`). `PUT /admin/posts/{id}`:
  1. `strconv.Atoi(chi.URLParam(r, "id"))` — a non-integer id → **400** "Only integer ids allowed!".
  2. Decode the body into a `model.Post` — bad JSON → **400**.
  3. Same validation as `CreatePost` (→ **422**) and the same nil-`tags` → `[]string{}` fix.
  4. `UPDATE posts SET title=$1, slug=$2, excerpt=$3, content_md=$4, tags=$5, updated_at = now() WHERE id = $6 RETURNING id, created_at, updated_at, published_at, is_published`
     via `QueryRow`, scanned back into `post`.
  5. Errors: `pgx.ErrNoRows` (no post with that id) → **404**; `23505` (slug taken by another post) → **409**; else → 500.
  6. Success → **200** + the post. Because publish state and timestamps come from `RETURNING`, the response shows what's
     actually stored, not what the client sent.

  Does not touch `is_published` / `published_at` — that's the publish endpoints' job. Verified: `id 1` was updated through
  the API (new title/content, `updated_at` moved; `created_at`, `is_published`, `published_at` unchanged).
- **`DeletePost`** (done 2026-10-04, not yet committed). `DELETE /admin/posts/{id}`:
  1. `strconv.Atoi` on the `{id}` param — non-integer → **400** "Only Integer ids allowed!".
  2. `DELETE FROM posts WHERE id = $1` via **`h.DB.Exec`** (no rows needed back, so no `QueryRow`/`Scan`).
  3. `err != nil` → **500** (a real DB failure).
  4. Then, as a separate check: `RowsAffected() == 0` on the returned command tag → **404** "Nothing to delete".
  5. Otherwise → **204 No Content**, no body.

  It's a hard delete and doesn't check publish state — published posts can be deleted directly. Verified: posts `id 14` and
  `id 2` were deleted through the API, and deleting `id 14` a second time returned 404.

## What's stubbed (no DB access yet)
In `internal/http/handlers.go`:
- `PublishPost` — returns 501 Not Implemented.
- `UnPublishPost` — returns 501 Not Implemented.

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
  in `r`, and where do I write each thing the frontend *reads* to `w`. `temp.mts` in the repo root (committed) holds whichever
  request is currently being tested — run it with `node temp.mts`.
- **Every handler has the same four steps**: parse the input → validate it → do the DB work → respond.
  `GetPosts` has no input; `GetPostBySlug` parses a URL param; `CreatePost` is the first that needs all four.
- **The backend sets the contract, and input is untrusted.** Frontend validation is UX; backend validation is the real guard,
  because requests can come from curl/Postman/anyone, not just your form.
- **Why a struct is needed to decode JSON**: JS `JSON.parse` gives a shapeless object; Go needs a container with named slots
  declared up front. `json.NewDecoder(r.Body).Decode(&post)` is Go's `JSON.parse` (the mirror of `Encode`, which is `JSON.stringify`).
  The `&` is for the same reason as in `Scan` — the decoder writes into your variable.
- **Missing JSON keys leave fields at their zero value** (`""`, `false`, `nil`) — like destructuring defaults in JS.
- **Struct fields must start with an uppercase letter** (exported) or `encoding/json` silently ignores them — no error, the
  field just stays empty. The JSON key can still be lowercase via the `json:"..."` tag.
- **One field, three names**: JSON key `content` ↔ Go field `ContentMD` (bridged by the `json` tag) ↔ column `content_md`
  (bridged by your SQL column list).
- **Bad JSON is the client's fault → 400**, not 500. 5xx means the server broke; 4xx means the request was wrong.
- **`INSERT ... RETURNING col1, col2`** makes Postgres send DB-generated values (id, default timestamps) back as a row, so
  you read them with `QueryRow` + `Scan`, exactly like a `SELECT` of one row.
- **SERIAL ids can have gaps** — a failed insert still uses up a sequence number.
- **Validation goes between decode and the DB call** (step 2 of parse → validate → work → respond), and each failed check
  responds and `return`s immediately, so invalid data never reaches the INSERT.
- **400 vs 422**: 400 Bad Request when the body can't be parsed at all; 422 Unprocessable Entity when it parses fine but the
  values break your rules. Both are 4xx because the client sent something wrong.
- **`strings.TrimSpace(x) == ""`** catches both empty and whitespace-only strings, which a plain `x == ""` misses.
- **`errors.Is` vs `errors.As`**: `errors.Is(err, target)` asks "is this *that specific* error value?" (e.g. `pgx.ErrNoRows`).
  `errors.As(err, &target)` asks "is this error *of that type*?" and, if so, fills `target` so you can read its fields.
  Both look through wrapped errors. Use `As` when you need details from inside the error.
- **Postgres error codes**: a `*pgconn.PgError` carries a 5-character SQLSTATE `Code`. `23505` = unique violation (→ 409).
  Other useful ones later: `23502` not-null violation, `23503` foreign-key violation.
- **A nil Go slice is sent to Postgres as NULL**; an empty non-nil slice (`[]string{}`) is sent as an empty array `{}`.
  Same nil-vs-empty distinction as the `null` vs `[]` JSON lesson from `GetPosts`, now on the database side.
- **A column's `DEFAULT` only applies when the column is left out of the INSERT.** Passing NULL explicitly overrides the
  default, so `tags TEXT[] NOT NULL DEFAULT '{}'` still rejects an explicit NULL.
- **Only failures inside Postgres use up ids.** Requests rejected by your Go validation (422) never reach the DB, so they
  don't leave gaps; duplicate-slug or NOT NULL failures do.

## Concepts learned building `UpdatePost` and the route split
- **URL params are always strings.** `chi.URLParam` returns a `string`; `strconv.Atoi` converts it to an `int` and returns an
  error for anything that isn't an integer — that error is the client's fault → 400.
- **`UPDATE ... WHERE id = $n RETURNING ...` + `pgx.ErrNoRows` → 404.** If no row matches the id, nothing is updated and
  `RETURNING` gives back zero rows, so `Scan` returns `pgx.ErrNoRows` — the same not-found signal as a `SELECT`.
  (The alternative, for statements without `RETURNING`: run them with `Exec` and check `RowsAffected() == 0`.)
- **PUT means full replace.** The client sends every editable field; a missing field becomes its zero value and would blank
  the column, which is why the same validation as create runs again. (A partial update would be PATCH.)
- **`updated_at` has to be set by hand in every UPDATE** (`updated_at = now()`). The column's `DEFAULT now()` only fires on
  INSERT; Postgres doesn't bump it on its own. (A trigger could automate this later.)
- **Return what's stored, not what was sent.** Scanning `published_at` / `is_published` from `RETURNING` means the response
  can't echo values the client sent but the DB ignored.
- **Publishing as separate action endpoints** (`POST .../publish`, `.../un-publish`) instead of an `is_published` field in
  create/update: changing *state* (draft → published) is a different operation from editing *content*, and it gives
  `published_at` exactly one place where it gets set.
- **Public vs admin routes.** Readers get `/posts` (published only); writing and seeing drafts lives under `/admin/posts`.
  The prefix also makes it easy to protect all admin routes at once later — but on its own it's just a naming convention,
  not security.
- **Different response shapes can use different structs.** `model.List` is the list-item shape (no `content`), the output
  equivalent of the "smaller input struct" idea.

## Concepts learned building `DeletePost`
- **pgx has three ways to run SQL**: `Query` (many rows back), `QueryRow` (one row back), `Exec` (no rows needed back —
  DELETE, or INSERT/UPDATE without `RETURNING`).
- **`Exec` returns `(pgconn.CommandTag, error)`.** The command tag is Postgres's summary of what the statement did — the same
  text psql prints, like `INSERT 0 1` or `DELETE 1`. `tag.RowsAffected()` reads the row count out of it (an `int64`).
- **`Scan` has two possible jobs**: getting values for the response, and detecting "no row" via `pgx.ErrNoRows`. A delete
  that responds 204 needs neither values nor `Scan` — only a yes/no on whether a row was removed, which `RowsAffected()` gives.
- **With `Exec`, "not found" is not an error.** A DELETE that matches nothing succeeds with `err == nil` and a count of 0.
  So the checks are **sequential, not nested**: `if err != nil` → 500 and return; then `if RowsAffected() == 0` → 404 and
  return; then 204. (Compare `QueryRow` + `Scan`, where "no row" arrives *as* an error and is checked inside `if err != nil`.)
  Frontend analogy: `fetch` only rejects on network failure; a 404 resolves normally and you check `response.status` yourself.
  `err` is the rejection, `RowsAffected()` is the status check.
- **DELETE is idempotent in effect, not in response**: deleting the same id twice leaves the DB in the same state, but the
  second call gets 404 because there was nothing left to delete.
- **204 means no body.** Calling `response.json()` on a 204 in JS throws "Unexpected end of JSON input".

## Open issues & review notes (pick up here)
1. **`GetPostBySlug` doesn't return the post body.** Its `SELECT` and `Scan` leave out `content_md`, so the single-post
   response has `"content": ""` — a post page would have nothing to render. (Present since 2026-07-12; missed in the earlier
   review.) Quick fix.
2. **`/admin/*` has no auth.** Anyone who can reach the server can create, edit, publish and delete posts. Must be fixed before
   the server is reachable from the internet. chi can group routes under one prefix and attach a middleware to just that group.
3. **`CreatePost`'s 201 response can contradict the DB.** It no longer stores `is_published`, but it still encodes the decoded
   `post`, so a body with `"is_published": true` gets a response saying `true` while the row is a draft. `UpdatePost` avoids
   this by scanning those columns from `RETURNING`; the same idea applies here (or a separate input struct, see #8).
4. **Validation is duplicated in `CreatePost` and `UpdatePost`.** The same checks are copy-pasted; if the rules change in one
   place and not the other, create and update will disagree. Candidate for a small shared helper function.
5. **Slug check is a blocklist** (rejects spaces and capitals). Other URL-unsafe characters still get through (`/`, `?`, `#`,
   `%`, `_`, accented letters, emoji, leading/trailing/double hyphens). An **allowlist** ("only lowercase letters, digits and
   single hyphens") covers all of them in one rule. General rule: describe what's allowed, not what's forbidden.
6. **Regexes are compiled on every request** (`regexp.MustCompile` inside the handlers — now in two places). Compile once at
   package level (a `var` outside the functions).
7. **Old rows are published without `published_at`**: `id 5`, `id 10`, `id 14` (created while `CreatePost` still accepted
   `is_published`). Fix them through the publish endpoint once it exists, or with a one-off SQL update.
8. **Decoding straight into `model.Post` lets the client send fields it shouldn't** (`id`, `created_at`, `published_at`,
   `is_published`, ...). The SQL ignores them, but see #3. The clean fix is a separate input struct with only the editable fields.
9. **`Excerpt` is a plain `string` but the column is nullable** (in both `model.Post` and `model.List`). The API always writes
   `""`, never NULL, so this only bites if a NULL gets in another way (e.g. a manual SQL insert) — then `Scan` errors with 500.
10. **`GetPosts` and `GetPostsInternal` duplicate the whole scan loop** — only the `WHERE` differs. Optional cleanup.
11. **`temp.mts` calls `await response.json()` on a 204**, which throws because a 204 has no body — a successful delete
    looks like a crash. (It caused the delete test to be run twice.) Log a message instead of parsing JSON on 204.
12. **`DeletePost` names the command tag `row`.** `Exec` returns a command tag, not a row; a name like `result` reads better
    when calling `.RowsAffected()` on it. Cosmetic.

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
Rows in the `blog_svc` DB (inside the Docker container, `pgdata` volume) as of 2026-10-04:
- `id 1` — slug `hello-world`, published, `published_at` set (inserted by hand via psql). Title/content were changed to
  "Andi mandi sandi" / "Lorem ipsum dolor" through `PUT /admin/posts/1`.
- `id 3` — slug `Second post from-the-client-side`, draft — **invalid slug** (spaces/uppercase), created before validation
  existed; kept as an example.
- `id 5` — slug `Second-post-from-the-client-side`, published, `published_at` NULL (Open issues #7).
- `id 10` — slug `hello-world-2`, published, `tags` `{}`, `published_at` NULL (Open issues #7).

Deleted through `DELETE /admin/posts/{id}`: `id 2` and `id 14`.
Never existed: ids 4, 6–9, 11–13 — inserts that failed inside Postgres (duplicate slugs, and NULL `tags` before the fix).

## Next steps (in order)
1. ~~Confirm DB is actually reachable and migrated~~ — **done**.
2. ~~`GetPosts`~~ — **done**.
3. ~~`GetPostBySlug`~~ — **done**, but see Open issues #1 (missing content) — quick fix worth doing first.
4. ~~`CreatePost`~~ — **done** (passes 1–3 + tags fix; now always creates drafts).
5. ~~`UpdatePost`~~ — **done 2026-10-04**.
6. ~~`DeletePost`~~ — **done 2026-10-04** (done before publish/unpublish).
7. **`PublishPost` / `UnPublishPost`** ← **pick up here next** — the last two stubs. Decisions to make before writing:
   - Publish sets `is_published = true` and `published_at`. If a post is unpublished and then published again, should
     `published_at` keep the original date or reset to now?
   - Does unpublishing clear `published_at` or leave it?
   - Publishing an already-published post: error (409) or quietly succeed (200)?
   - Should these bump `updated_at`?
   - Unknown id → 404. Either pattern works: `RETURNING` + `pgx.ErrNoRows` (like `UpdatePost`) if the response should
     include the updated post, or `Exec` + `RowsAffected()` (like `DeletePost`) if it shouldn't.
   - Once working, use it to fix the old rows in Open issues #7.
8. **Cross-cutting, after basic CRUD works**
   - Auth for `/admin/*` (Open issues #2) — needed before this is internet-facing.
   - Shared validation helper + slug allowlist + package-level regex (Open issues #4–#6).
   - Consistent error response shape (JSON error body instead of plain text).
   - Per-request DB timeouts (`context.WithTimeout` around `r.Context()`).
   - Basic tests (handler tests with a test DB, or integration tests against the docker-compose Postgres).
   - Pagination for `GET /posts` once there are more posts.
   - Read the listen port from a `PORT` env var (default `8080`) instead of hardcoding `:8080` in `cmd/api/main.go`.

## How to resume this in a new session
Point Claude at this file and say "read PROGRESS.md and tell me what's next." The "API routes" table shows what's done and
what's stubbed; "Open issues" lists known problems; "Next steps" says where to pick up. Update all three as items get done.
