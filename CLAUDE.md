# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go reimplementation of a Jellyfin media server, serving the Jellyfin 10.10.0 HTTP API so that stock `jellyfin-web` and Jellyfin clients can talk to it.

## Rules

**Comments.** Don't write them. A comment is almost always a sign the code is messy or doing too much — simplify the code instead of explaining it. The rare exception is an external constraint that can't live in a name (why the mux is regex-based, why the websocket pings at 30s).

**Paradigms.** Use what the codebase already does. Don't introduce a new pattern or a new dependency if the existing ones can carry the change, and never refactor toward a better paradigm mid-change. If something genuinely needs rework, add a line to `TODO.md` and move on.

**Commits.** Always be committing — don't wait to be asked, and never leave finished work sitting in the working tree. Commit as soon as a change builds and boots, then keep going. Small and iterative: each commit stands on its own and can be applied or replayed without depending on a later commit to make it valid. Migrations in particular must be safe to re-run.

## Commands

```sh
go run ./cmd/server                  # run (listens on :8081)
go build ./...
go test ./...
go test -run TestName ./internal/... # single test
go generate ./internal/server/api    # regenerate API from the OpenAPI spec
air                                  # watch and restart (go install github.com/air-verse/air@latest)
```

`go generate` is deliberately not part of the watch loop: it re-emits ~95k lines and only matters when the spec or `internal/server/api/gen` changes.

`air` owns `:8081` while it runs, so starting a second server alongside it fails with `ListenAndServe error: address already in use`. Check whether it is running with `pgrep -x air` (matching on a path fails — the process is just `air`), and the listener with `lsof -ti:8081 -sTCP:LISTEN` — without `-sTCP:LISTEN` it also matches browsers connected to the port, and killing those results is not what you want. An orphaned `.air/server` can outlive its supervisor and keep serving stale code.

Requires a reachable Postgres. `DATABASE_URL` overrides the default DSN in `internal/store/store.go` (`postgres://localhost:5432/gojellyfin_development?sslmode=disable`).

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

### Domain services

Three layers, split on what each is allowed to know:

- **`internal/{auth,users,items,libraries,config}` — domains.** Models and behaviour, exposing a `Service` built with `New(db *gorm.DB)`. **A domain package must never import `internal/server/api` or `internal/http/middleware`.** That invariant is what the layout rests on; check it with `grep -rl 'server/api\|http/middleware' internal/<domain>/`, which must come back empty. `auth` owns sessions, hashing and tokens; `users` owns only the user record.
- **`internal/server/<tag>` — one package per spec tag.** Named for the tag (`userlibrary`, `librarystructure`, `mediainfo`), holding exactly the operations that tag declares and a `Server` with the domain services it needs. Add a tag package by looking the operation up in the spec, not by guessing where it feels like it belongs — `AuthenticateUserByName` is a `User` operation, `GetBitrateTestBytes` is `MediaInfo`.
- **`internal/server/dtos`** — translation and helpers shared by more than one tag (`ItemDto`, `UserDto`, `SessionDto`, `ServerConfiguration`, plus `Ptr`/`Deref`/`Body`/`UID`). Domains can't hold these because they'd have to import `api`.

Two naming traps, both of which cost real time: generated operation names occupy the method namespace, so domain queries need distinct names (`ItemByID`, not `GetItem`); and package names are easily shadowed by locals — `items []items.Item`, `dtos := make(...)`. Name the local something else.

`internal/server` itself is only the composition root plus the transport edge (`api`, `socket`, `stream`).

Cross-domain wiring uses interfaces declared by the *consumer* — `middleware.Sessions`, `libraries.Scanner`. Where that would make the object graph cyclic (libraries needs the scanner, the scanner reads libraries), the dependency is set after construction via `fx.Invoke` rather than in the constructor.

`server.Server` embeds each service, and embeds `api.Unimplemented` **one level deeper** through `nestedUnimplemented`. Go resolves a method at the shallowest depth where exactly one candidate exists, so a service at depth 1 wins and everything unimplemented falls through to the 501 stub at depth 2. Three things silently break this, all surfacing as a confusing "does not implement StrictServerInterface": a service embedding `api.Unimplemented` itself, two services declaring the same method, or flattening the wrapper. Embedded field names are type names, so each service is embedded through a local alias (`type UsersServer = users.Server`) to keep them distinct.

### Persistence

`internal/store` owns the connection (`NewDB`), migrations, and the `JSON` type for `jsonb` columns. That's all — there is no `Store` interface and no shared repository.

Renaming a model does **not** rename its table: gorm infers the table name from the type, so a renamed model needs an explicit `TableName()` (see `items.Datum` → `user_item_data`).

Schema changes go through `internal/store/migrations`: add `000N_description.go` defining a `*gormigrate.Migration` and append it to `all` in `migrations.go`. Migrations run transactionally at startup via `fx.Invoke(migrations.Run)`. Migration files snapshot their models as local structs rather than referencing the live model, which is what lets models move between packages without touching them.

Columns owned by a background process must be left out of an upsert's `DoUpdates` — the scan clobbering probe-owned columns like `run_time_ticks` was a real bug.

### Request identity

`internal/auth` owns identity on the context — it puts it on and takes it off, and nothing else knows the keys exist. `middleware.Auth` only parses what the client sent (`Authorization: MediaBrowser …`, `X-Emby-Token`, `?api_key=`) and calls `auth.Authenticate`, which resolves the token and returns a context carrying the session.

Handlers read `auth.UserID(ctx)`, `auth.SessionFrom(ctx)`, `auth.AuthorizationFrom(ctx)` and return `auth.ErrUnauthorized`; `middleware.TokenFrom` is the only thing left in the middleware package that handlers touch, and only because websocket and media URLs cannot send headers.

### Current state

Most handlers return hardcoded data — `adminUser()` in `internal/server/user.go` is a single fake admin, and auth accepts anything. The store is wired but not yet used by handlers.
