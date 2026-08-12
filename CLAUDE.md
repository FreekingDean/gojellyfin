# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go reimplementation of a Jellyfin media server, serving the Jellyfin 10.10.0 HTTP API so that stock `jellyfin-web` and Jellyfin clients can talk to it.

## Rules

**Comments.** Don't write them. A comment is almost always a sign the code is messy or doing too much — simplify the code instead of explaining it. The rare exception is an external constraint that can't live in a name (why the mux is regex-based, why the websocket pings at 30s).

**Paradigms.** Use what the codebase already does. Don't introduce a new pattern or a new dependency if the existing ones can carry the change, and never refactor toward a better paradigm mid-change. If something genuinely needs rework, open a GitHub issue with `gh issue create` and move on. Deferred work is tracked in issues only — there is no TODO file. Write the issue for someone reading it in six months: the concrete detail and the file or symbol it concerns, not a one-line reminder. Decisions get filed the same way, worded as the standing decision and its reason so nobody re-litigates them.

**Pull requests.** Never commit to `main`. Every change goes on a branch and lands through a PR for review, including one-line fixes and documentation. Push the branch and open the PR with `gh pr create` as soon as the work builds and boots.

**Worktrees.** Work happens in a worktree under `.worktrees/`, never in the main checkout — several branches are usually in flight at once. Create one with `git worktree add -b <branch> .worktrees/<name> origin/main`. The main checkout is reserved for tracking `origin/main`; don't develop or commit there.

**Commits.** Always be committing — don't wait to be asked, and never leave finished work sitting in the working tree. Commit as soon as a change builds and boots, then keep going. Small and iterative: each commit stands on its own and can be applied or replayed without depending on a later commit to make it valid. Migrations in particular must be safe to re-run.

## Commands

```sh
make dev                             # watch and restart (go install github.com/air-verse/air@latest)
make run                             # run once, no watching
make build test fmt
make generate                        # regenerate the API from the spec and the store from the ent schema
go test -run TestName ./internal/... # single test

echo hunter2 | go run ./cmd/tasks/adduser -name Dean   # bootstrap the first user
```

`make dev` and `make run` tee to `/tmp/gojellyfin.log`, so the log is on screen and readable by tooling at the same time.

`build` depends on `generate`, and `run` and `test` depend on `build`, so the generated code is never stale. The watch loop is the exception: `air` only builds, because regenerating re-emits ~95k lines on every save.

Run `air` through `tee` so the log is both on screen and readable at `/tmp/gojellyfin.log`; the request log is the fastest way to find what a client actually calls.

`air` owns `:8081` while it runs, so starting a second server alongside it fails with `ListenAndServe error: address already in use`. Check whether it is running with `pgrep -x air` (matching on a path fails — the process is just `air`), and the listener with `lsof -ti:8081 -sTCP:LISTEN` — without `-sTCP:LISTEN` it also matches browsers connected to the port, and killing those results is not what you want. An orphaned `.air/server` can outlive its supervisor and keep serving stale code.

Requires a reachable Postgres. `DATABASE_URL` overrides the default DSN in `internal/store/fx.go` (`postgres://localhost:5432/gojellyfin_development?sslmode=disable`).

`make test` needs one too — `internal/items` seeds real rows through `store.NewStore()` and fails rather than skipping when the database is unreachable, so a green run means the queries actually ran. Each test owns a library row and deletes it and its items on cleanup; point `DATABASE_URL` at a scratch database to keep development data out of it. CI runs the suite against a `postgres:16` service with `internal/store/migrations` applied by `atlas migrate apply`.

Nothing migrates at startup. Schema changes mean editing `internal/store/entities`, running `make generate`, then generating and applying the SQL by hand from `internal/store` (the `atlas migrate diff` line in `generate.go` is commented out because it needs Docker):

```sh
atlas migrate diff <name> --dir "file://migrations" --to "ent://entities" --dev-url "docker://postgres/16/dev?search_path=public"
atlas migrate apply --dir "file://migrations" --url "$DATABASE_URL"
```

## Architecture

### Wiring: uber/fx

Every package under `internal/` exposes a `Module` in its own `fx.go`; `cmd/server/main.go` composes them. New packages follow the same shape: `New` constructor in the package, `fx.Provide`/`fx.Invoke` in `fx.go`, `fx.Lifecycle` hooks in a `run` function for anything with start/stop semantics (see `internal/http/fx.go`, `internal/store/fx.go`).

### API layer: generated, with an unimplemented base

`internal/server/api` is entirely generated and should never be hand-edited. `go:generate` runs two steps:

1. `oapi-codegen` (strict server + models + embedded spec) → `jellyfinapi.gen.go`, a ~95k-line file defining `StrictServerInterface` with one method per Jellyfin endpoint.
2. `./gen` (a local AST tool) reads that interface and emits `unimplemented.gen.go` — an `Unimplemented` struct implementing every method with `ErrNotImplemented`.

`server.Server` embeds `api.Unimplemented`, so it satisfies the full interface while only the endpoints actually written exist as real methods. **Implementing an endpoint means adding a method to `internal/server` that shadows the embedded stub.** `ErrNotImplemented` is mapped to HTTP 501 by the response error handler in `internal/http/http.go`.

`spec/jellyfin-openapi-10.10.0.json` is the vendored upstream spec, locally patched with `x-go-type` overrides where upstream types generate badly. Re-vendoring the spec means re-applying those patches.

### Request path

`internal/http` owns the `net/http` server and composes two middleware layers: `middleware.HttpMiddleware` (stdlib handler wrappers — CORS, logging) applied outside, and `api.StrictMiddlewareFunc` (operation-aware, has the operation ID) applied inside the generated handler.

Routing uses a hand-rolled `internal/http/mux`, not `http.ServeMux`, because Jellyfin clients hit case-insensitive paths and paths with literal dots (`stream.mp4`). Patterns compile to regexes: `{name}` captures a segment, `*` is a catch-all, matching is case-insensitive, and a trailing slash is always tolerated. Path params land via `r.SetPathValue`.

`GET /socket` is registered outside the generated API for the websocket keepalive loop (`internal/server/socket`).

So are routes Jellyfin serves but hides from its own OpenAPI document with `[ApiExplorerSettings(IgnoreApi = true)]`. There are 36 of them, almost all the pre-10.9 `/Users/{userId}/…` spellings, and no version of the spec contains any of them — searching a newer spec will not find them either. `spec/jellyfin-hidden-routes-10.10.0.txt` is the extracted list, and `TestLegacyRoutesCoverJellyfin` fails if one of them has neither an alias nor a stated reason. `jellyfin-web` still calls some, so the symptom is a 404 for a path the spec does not define; the definition is in the controller source (`GET /Users/{userId}/Items/{itemId}` is `Jellyfin.Api/Controllers/UserLibraryController.cs`).

`legacyRoutes` in `internal/http/http.go` maps each to its documented spelling and re-dispatches through the mux, so an alias costs a table entry rather than a handler. `[Obsolete]` alone does **not** mean missing — most obsolete routes are still documented and already generated; only `IgnoreApi` hides them.

They register after the generated routes so a documented literal wins any overlap, and most-specific first: the mux matches in registration order with no notion of specificity, so `/Users/{userId}/Items/{itemId}` registered before `/Users/{userId}/Items/Resume` swallows it and passes "Resume" as an item id. `legacyPatterns` sorts by literal segments to keep that from depending on map order.

### Domain services

Three layers, split on what each is allowed to know:

- **`internal/{auth,sessions,users,items,libraries,config}` — domains.** Behaviour over the ent models, exposing a `Service` built with `New(client *store.Client)`. **A domain package must never import `internal/server/api` or `internal/http/middleware`.** That invariant is what the layout rests on; check it with `grep -rl 'server/api\|http/middleware' internal/<domain>/`, which must come back empty. `auth` owns hashing, tokens and request identity; `sessions` owns the active session and device rows, which are Jellyfin sessions rather than login state.
- **`internal/server/<tag>` — one package per spec tag.** Named for the tag (`userlibrary`, `librarystructure`, `mediainfo`), holding exactly the operations that tag declares and a `Server` with the domain services it needs. Add a tag package by looking the operation up in the spec, not by guessing where it feels like it belongs — `AuthenticateUserByName` is a `User` operation, `GetBitrateTestBytes` is `MediaInfo`.
- **`internal/server/apiutil`** — five generic helpers (`Ptr`, `Deref`, `OrElse`, `Body`, `UID`) and nothing else. It imports none of our packages, and no domain knowledge belongs in it; Go cannot alias generic functions, which is the only reason they are shared rather than copied.

**A tag package never imports another tag package.** Translation two or more tags need lives in **`internal/server/dto`** — `ItemDto`/`ItemDtos`/`LibraryView`/`UserItemDataDto`/`Kinds`/`SessionDto`/`CultureDtos`/`CountryInfos`/`ParentalRatings` today, in both directions, model→DTO and DTO→model. It may import `api`, `apiutil` and the domains, and it must never import a tag package. Domains cannot hold any of it, because they would have to import `api`.

Translation a single tag uses stays in that tag's `dto.go` and is **unexported** — `user.userDto`, `playlists.playlistDto`, `scheduledtasks.taskInfo`. That is what gives the rule its teeth: **no exported translation function in a tag package**, so there is nothing for a second tag to reach for, and the second caller is the moment a helper moves to `dto`. `TestTagPackagesKeepTranslationUnexported` in `internal/server/translation_test.go` parses every tag package and fails on an exported function whose signature names an `api` type. Its exception table is the whole of the escape hatch: `configuration.ServerConfiguration`/`BrandingConfiguration` read stored config rather than translate. Adding a name there needs a reason, and a stale entry fails the test too.

Two naming traps, both of which cost real time: generated operation names occupy the method namespace, so domain queries need distinct names (`ItemByID`, not `GetItem`); and package names are easily shadowed by locals — `items []items.Item`, `dtos := make(...)`. Name the local something else.

`internal/server` itself is only the composition root plus the transport edge (`api`, `socket`, `stream`).

Cross-domain wiring uses interfaces declared by the *consumer* — `middleware.Sessions`. Where taking the dependency as a constructor argument would make the object graph cyclic, it is set after construction via `fx.Invoke`: `registerLibraryScan` in `internal/server/fx.go` hands `scanner.Scan` to the task registry as the `tasks.LibraryScanID` runner and appends the lifecycle hook that starts it at boot, so the startup scan is the same task the dashboard and `RefreshLibrary` start.

`server.Server` embeds each service, and embeds `api.Unimplemented` **one level deeper** through `nestedUnimplemented`. Go resolves a method at the shallowest depth where exactly one candidate exists, so a service at depth 1 wins and everything unimplemented falls through to the 501 stub at depth 2. Three things silently break this, all surfacing as a confusing "does not implement StrictServerInterface": a service embedding `api.Unimplemented` itself, two services declaring the same method, or flattening the wrapper. Embedded field names are type names, so each service is embedded through a local alias (`type UsersServer = users.Server`) to keep them distinct.

### Persistence

`internal/store` is generated by ent from the schemas in `internal/store/entities`, and should never be hand-edited — `fx.go` and `txhelper.go` look hand-written but come from `internal/store/templates`, so changes go in the template and get regenerated. There is no `Store` interface and no shared repository; domains hold a `*store.Client` and alias the entities they own (`type Item = store.Item`).

Domains never expose an ent type through a name the api layer has to know. They alias the entity and its enums (`items.Kind`, `libraries.CollectionType`) so tag packages can build DTOs without importing `store`.

Edges that queries and DTOs read constantly declare their foreign key as a field (`edge.From("library", …).Field("library_id")` alongside `field.UUID("library_id", …)`). Without it ent keeps the column unexported, every read of a parent id costs an eager load, and upserts have no conflict target to name.

Anything a create has to supply on every call belongs in the schema as `.Default(...)` rather than in the caller — that is what keeps the ~40-field `UserPolicy` and `LibraryOptions` rows constructible from the domain, which cannot see the api defaults. Update paths take the opposite shape: ent generates `SetNillableX` for every field, which is exactly the pointer shape the api sends, so tag packages chain those onto a builder the domain hands back.

Columns owned by a background process must be left out of an upsert's `DoUpdates` — the scan clobbering probe-owned columns like `run_time_ticks` was a real bug. The scan writes `date_modified`; the probe writes `container`, `run_time_ticks`, `probed_at` and the `MediaSource`.

Ordering by a to-many edge makes ent group the query, so the sort column comes back unaggregated and Postgres rejects it. Query from the side that owns the column instead (see `items.ResumeItems`).

### Request identity

`internal/auth` owns identity on the context — it puts it on and takes it off, and nothing else knows the keys exist. `middleware.Auth` only parses what the client sent (`Authorization: MediaBrowser …`, `X-Emby-Token`, `?api_key=`) and calls `auth.Authenticate`, which resolves the token through `sessions` and returns a context carrying the session.

`auth.UserID` reads the session's user edge, so `sessions.ByToken` has to eager-load it; the foreign key is unexported and there is nothing to fall back on. Nothing else hangs off that query — an edge is eager-loaded when a caller reads it, not in anticipation of one, because `ByToken` runs on every authenticated request and each edge costs another round trip.

`auth.Authorization` carries the connection's `RemoteAddr` alongside what the client sent, because the strict handlers never see the `*http.Request` and `GetEndpointInfo` answers from the caller's address.

`ForgotPassword` answers `ContactAdmin` and `ForgotPasswordPin` refuses every pin, both without reading a request or touching the database. A pin is only as private as the channel that carries it, and the server has none — no mail, and the log and the pin file upstream writes are both read by whoever runs the box rather than by the account holder. Anything that issues a pin here hands account takeover to everyone who can read a shipped log. `cmd/tasks/resetpassword` is what `ContactAdmin` means: an operator with database access, reading the new password from stdin the way `adduser` does.

Handlers read `auth.UserID(ctx)`, `auth.SessionFrom(ctx)`, `auth.AuthorizationFrom(ctx)` and return `auth.ErrUnauthorized`; `middleware.TokenFrom` is the only thing left in the middleware package that handlers touch, and only because websocket and media URLs cannot send headers.

### Current state

Users, sessions, devices, libraries, items and their user data are real rows, as are playlists, their entries and their shares; most other handlers still return hardcoded data or a 501.

A fresh database has no way in through the API — `CreateUserByName` requires an administrator and nothing seeds one — so `cmd/tasks/adduser` creates the first one. It reads the password from stdin rather than a flag, which keeps it out of the shell history and the process list. One-off jobs that need the domain services rather than a running server belong beside it under `cmd/tasks`.
