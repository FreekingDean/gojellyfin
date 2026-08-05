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
```

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

Handlers are being moved out of `package server` into a package per domain, keyed on the spec's tags (every operation carries exactly one tag). `internal/server/users` is the template: handlers, the gorm model and its queries all live together, with `New(db *gorm.DB)`. No interface between handler and query — they're the same package. Files split by concern (`users.go`, `store.go`, `auth.go`, `session.go`), not by layer; there is no separate "service" type, because most operations are translate-and-query and a forwarding layer would be noise.

`server.Server` embeds each service, and embeds `api.Unimplemented` **one level deeper** through `nestedUnimplemented`. Go resolves a method at the shallowest depth where exactly one candidate exists, so a service at depth 1 wins and everything unimplemented falls through to the 501 stub at depth 2. Three things silently break this, all surfacing as a confusing "does not implement StrictServerInterface": a service embedding `api.Unimplemented` itself, two services declaring the same method, or flattening the wrapper. Embedded field names are type names, so each service is embedded through a local alias (`type UsersServer = users.Server`) to keep them distinct.

### Persistence

`internal/store` owns the connection (`NewDB`), migrations, and shared types like `JSON`. It still holds the not-yet-extracted domains behind the `Store` interface; that interface shrinks as each domain moves to its own package.

Schema changes go through `internal/store/migrations`: add `000N_description.go` defining a `*gormigrate.Migration` and append it to `all` in `migrations.go`. Migrations run transactionally at startup via `fx.Invoke(migrations.Run)`. Migration files snapshot their models as local structs rather than referencing the live model, which is what lets models move between packages without touching them.

Columns owned by a background process must be left out of an upsert's `DoUpdates` — the scan clobbering probe-owned columns like `run_time_ticks` was a real bug.

### Request identity

`middleware.Auth` resolves a token through the narrow `middleware.Sessions` interface and puts a `middleware.Session` (user id, session id) in the context — not the user record, which would make every domain package a dependency of the middleware. Handlers read `middleware.UserID(ctx)` and fetch what they need.

### Current state

Most handlers return hardcoded data — `adminUser()` in `internal/server/user.go` is a single fake admin, and auth accepts anything. The store is wired but not yet used by handlers.
