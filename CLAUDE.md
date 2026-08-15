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

echo hunter2 | go run ./cmd/gojellyfin adduser Dean   # bootstrap the first user
```

Everything ships as one cobra binary, `cmd/gojellyfin`, with a subcommand each for `server`, `worker`, `migrate`, `adduser`, `resetpassword` and `localizationdata`. One binary means one image, so an operator task is `docker exec <container> gojellyfin adduser Dean` rather than a second image or a second build. The names stay flat and read as commands; a new task is a `<name>Command() *cobra.Command` constructor in a file of its own, added to the list in `main.go`.

Only two things are shared between them, both in `main.go`: `withStore` opens the store, starts it and closes it around a callback, and `readPassword` reads from stdin. Neither the DSN nor its default lives there — `store.DatabaseURL()` is the single source, which is why `migrate` can name the same database the server will use without repeating the string.

`make dev` and `make run` tee to `/tmp/gojellyfin.log`, so the log is on screen and readable by tooling at the same time.

`build` depends on `generate`, and `run` and `test` depend on `build`, so the generated code is never stale. The watch loop is the exception: `air` only builds, because regenerating re-emits ~95k lines on every save.

Run `air` through `tee` so the log is both on screen and readable at `/tmp/gojellyfin.log`; the request log is the fastest way to find what a client actually calls.

`air` owns `:8081` while it runs, so starting a second server alongside it fails with `ListenAndServe error: address already in use`. Check whether it is running with `pgrep -x air` (matching on a path fails — the process is just `air`), and the listener with `lsof -ti:8081 -sTCP:LISTEN` — without `-sTCP:LISTEN` it also matches browsers connected to the port, and killing those results is not what you want. An orphaned `.air/gojellyfin` can outlive its supervisor and keep serving stale code.

Requires a reachable Postgres. `DATABASE_URL` overrides the default DSN in `internal/store/fx.go` (`postgres://localhost:5432/gojellyfin_development?sslmode=disable`).

`make test` needs one too — `internal/items` seeds real rows through `store.NewStore()` and fails rather than skipping when the database is unreachable, so a green run means the queries actually ran. Each test owns a library row and deletes it and its items on cleanup; point `DATABASE_URL` at a scratch database to keep development data out of it. CI runs the suite against a `postgres:16` service with `internal/store/migrations` applied by `atlas migrate apply`.

Nothing migrates at startup. Schema changes mean editing `internal/store/entities`, running `make generate`, then generating and applying the SQL by hand from `internal/store` (the `atlas migrate diff` line in `generate.go` is commented out because it needs Docker):

```sh
atlas migrate diff <name> --dir "file://migrations" --to "ent://entities" --dev-url "docker://postgres/16/dev?search_path=public"
atlas migrate apply --dir "file://migrations" --url "$DATABASE_URL"
```

`gojellyfin migrate` is the deployed spelling of that second line. It drives the `atlas` CLI rather than the Go SDK because the revision tracker that owns `atlas_schema_revisions` is not in the `ariga.io/atlas` module — it lives in the CLI repo under `cmd/atlas/internal/migrate/ent`, which nothing outside that repo can import. The migration directory is embedded through `internal/store/migrations`, so a deployed binary cannot drift from the schema it expects, and `atlas.sum` is still verified on every run.

The `Dockerfile` carries that one binary and the `atlas` binary, runs as a non-root user, and is built and pushed to `ghcr.io/freekingdean/gojellyfin` by `.github/workflows/docker.yml`. `CMD` is `server`, so the image serves by default and any other subcommand is a `docker run` argument — a worker is the same image run as `worker`, which is the whole reason background work is a workflow rather than a goroutine in the server. `entrypoint.sh` survives the consolidation for one reason: it migrates first when `MIGRATE_ON_START=true`. That has to stay outside the binary, because it is deploy policy rather than server behaviour — unconditional migration on start is unsafe with rolling replicas, so the default is a one-off `docker run --rm <image> migrate`.

## Architecture

### Wiring: uber/fx

Every package under `internal/` exposes a `Module` in its own `fx.go`; `serverModules` in `cmd/gojellyfin/server.go` composes them into the one `fx.New(...).Run()` that the `server` subcommand is. fx wires the long-running commands, not the CLI — `adduser` and the rest build what they need by hand, because a one-shot task does not want a lifecycle. `server` and `worker` each compose their own graph. New packages follow the same shape: `New` constructor in the package, `fx.Provide`/`fx.Invoke` in `fx.go`, `fx.Lifecycle` hooks in a `run` function for anything with start/stop semantics (see `internal/http/fx.go`, `internal/store/fx.go`).

The command lists the domains one by one, but not the forty tag packages: `server.Module` aggregates those, so a tag module is named beside its thirty-nine siblings rather than in the composition root, and the command reads as the domains plus the API surface they are served through. Tag modules are named `server/<tag>` because five of them share a package name with a domain (`items`, `playlists`, `localization`, `displaypreferences`, `system`) and fx prints the module name in its graph and its errors.

A module aggregates another only where the second is part of the first's own edge, not as a shortcut past the command: `server.Module` takes the tag packages, and `http.Module` takes `mux` and `transcode` along with `middleware.NewAuth`, `socket.New` and `stream.New`, because those are the transport it owns. Each subcommand still composes its own list — a transcode pod is the `server` command too, so there is no smaller graph to compose — the split is in the routing.

A binding that exists only because the object graph would otherwise be cyclic goes in the `fx.go` of the package that supplies the dependency, not the one that consumes it: `scanner`'s `register` hands its `LibraryScan` to the `jobs.Registry`, so what the scanner is hooked into is answered by reading `internal/scanner/fx.go`.

`TestServerModulesResolve` in `cmd/gojellyfin/server_test.go` runs `fx.ValidateApp` over `serverModules`. A forgotten `Module` compiles cleanly — nothing references one except the list it belongs to — so the build cannot catch it and the server dies on start instead. `ValidateApp` checks the graph without opening a database or binding a port; dropping `mediainfo.Module` fails it with `missing type: *mediainfo.Server`.

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

- **`internal/{auth,sessions,users,items,libraries,tasks,config}` — domains.** Behaviour over the ent models, exposing a `Service` built with `New(client *store.Client)`. **A domain package must never import `internal/server/api` or `internal/http/middleware`.** That invariant is what the layout rests on; check it with `grep -rl 'server/api\|http/middleware' internal/<domain>/`, which must come back empty. `auth` owns hashing, tokens and request identity; `sessions` owns the active session and device rows, which are Jellyfin sessions rather than login state; `tasks` owns the workflows the dashboard drives.
- **`internal/server/<tag>` — one package per spec tag.** Named for the tag (`userlibrary`, `librarystructure`, `mediainfo`), holding exactly the operations that tag declares and a `Server` with the domain services it needs. Add a tag package by looking the operation up in the spec, not by guessing where it feels like it belongs — `AuthenticateUserByName` is a `User` operation, `GetBitrateTestBytes` is `MediaInfo`.
- **`internal/server/apiutil`** — five generic helpers (`Ptr`, `Deref`, `OrElse`, `Body`, `UID`) and nothing else. It imports none of our packages, and no domain knowledge belongs in it; Go cannot alias generic functions, which is the only reason they are shared rather than copied.

**A tag package never imports another tag package.** Translation two or more tags need lives in **`internal/server/dto`** — `ItemDto`/`ItemDtos`/`LibraryView`/`UserItemDataDto`/`Kinds`/`SessionDto`/`CultureDtos`/`CountryInfos`/`ParentalRatings` today, in both directions, model→DTO and DTO→model. It may import `api`, `apiutil` and the domains, and it must never import a tag package. Domains cannot hold any of it, because they would have to import `api`.

Translation a single tag uses stays in that tag's `dto.go` and is **unexported** — `user.userDto`, `playlists.playlistDto`, `scheduledtasks.taskInfo`. That is what gives the rule its teeth: **no exported translation function in a tag package**, so there is nothing for a second tag to reach for, and the second caller is the moment a helper moves to `dto`. `TestTagPackagesKeepTranslationUnexported` in `internal/server/translation_test.go` parses every tag package and fails on an exported function whose signature names an `api` type. Its exception table is the whole of the escape hatch: `configuration.ServerConfiguration`/`BrandingConfiguration` read stored config rather than translate. Adding a name there needs a reason, and a stale entry fails the test too.

Two naming traps, both of which cost real time: generated operation names occupy the method namespace, so domain queries need distinct names (`ItemByID`, not `GetItem`); and package names are easily shadowed by locals — `items []items.Item`, `dtos := make(...)`. Name the local something else.

`internal/server` itself is only the composition root plus the transport edge (`api`, `socket`, `stream`).

Cross-domain wiring uses interfaces declared by the *consumer*, and where that would make the object graph cyclic the dependency is set after construction via `fx.Invoke` rather than in the constructor — see Wiring above for where those invokes live.

`server.Server` embeds each service, and embeds `api.Unimplemented` **one level deeper** through `nestedUnimplemented`. Go resolves a method at the shallowest depth where exactly one candidate exists, so a service at depth 1 wins and everything unimplemented falls through to the 501 stub at depth 2. Three things silently break this, all surfacing as a confusing "does not implement StrictServerInterface": a service embedding `api.Unimplemented` itself, two services declaring the same method, or flattening the wrapper. Embedded field names are type names, so each service is embedded through a local alias (`type UsersServer = users.Server`) to keep them distinct.

### Persistence

`internal/store` is generated by ent from the schemas in `internal/store/entities`, and should never be hand-edited — `fx.go` and `txhelper.go` look hand-written but come from `internal/store/templates`, so changes go in the template and get regenerated. There is no `Store` interface and no shared repository; domains hold a `*store.Client` and alias the entities they own (`type Item = store.Item`).

Domains never expose an ent type through a name the api layer has to know. They alias the entity and its enums (`items.Kind`, `libraries.CollectionType`) so tag packages can build DTOs without importing `store`.

Edges that queries and DTOs read constantly declare their foreign key as a field (`edge.From("library", …).Field("library_id")` alongside `field.UUID("library_id", …)`). Without it ent keeps the column unexported, every read of a parent id costs an eager load, and upserts have no conflict target to name.

Anything a create has to supply on every call belongs in the schema as `.Default(...)` rather than in the caller — that is what keeps the ~40-field `UserPolicy` and `LibraryOptions` rows constructible from the domain, which cannot see the api defaults. Update paths take the opposite shape: ent generates `SetNillableX` for every field, which is exactly the pointer shape the api sends, so tag packages chain those onto a builder the domain hands back.

Ent's default delete action is `NO ACTION` for a required edge and `SET NULL` for an optional one, so a child row either blocks its parent's delete or is left orphaned with a dangling column. Owned rows carry `.Annotations(cascadeOnDelete)`, which only takes effect on the `edge.To` side — ent skips the inverse edge when it builds the foreign key, so on the self-referencing `children`/`parent` edge the annotation goes before `.From("parent")`. Deleting a library therefore takes its items with it, and a user takes their sessions and watch state; only activity log entries are left behind, keeping their history when the user or item goes away.

**An `Item` is a title; a `MediaSource` is a file.** The item carries no path — its identity is `key`, unique with `library_id`, derived by the scanner from the name and year (`movie:the-matrix:1999`, `series:the-wire`, `season:the-wire:1`, `episode:the-wire:1:3`). Keying on location meant a moved file changed identity and a reorganised library lost every resume position and play count; a name-derived key survives the move. The key is internal — no endpoint takes one in its path and the 10.10.0 spec has no field to put one in, so clients only ever see the UUID. `MediaSource` owns the path, unique per library, and an item may have many, which is how a 4K and a 1080p rip of one film become one entry with two versions.

The key is the whole of the grouping: two files whose names parse the same are two copies of one title, and a cut that is named differently on disk already parses to a key of its own. Nothing reads the runtime to second-guess that, so the scan needs no ordering between probing a file and deciding which item it belongs to.

`probed_at` and `date_modified` live on the source, so an unchanged file costs no ffprobe.

An item with more than one source has to pick one, and the two callers want opposite things. A stream takes `items.PreferredSource`, which is **compatibility over quality** — the best encode a client cannot decode is worse than the worse one it can — and falls back to the richest source when nothing matches, so the caller can still remux it or refuse with a status rather than handle a nil. A download takes `items.BestSource`, ignoring compatibility entirely, because the bytes are being saved rather than decoded and what the far end can play is its own problem. Matching is on container and audio codec only; a kind the client said nothing about is unrestricted rather than refused, so only a browser — which declares nothing — is held to the tables in `internal/server/stream`. Video codec is deliberately not part of it (#532), which is consistent with video being direct play only (#481).

Columns owned by a background process must be left out of an upsert's `DoUpdates` — the scan clobbering probe-owned columns like `run_time_ticks` was a real bug. `SaveScanned` writes the item's `date_modified` and `SaveSource` the source's; the probe writes `container`, `run_time_ticks`, `size`, `bitrate`, `probed_at`, `tags`, the source's streams and the genre, studio and credit edges, and rewrites all of them on every run, so the edges are cleared and re-added rather than merged.

The sweep is split the same way. Items are swept by key and **soft** deleted, because the row carries the id the watch state hangs off and a returning title has to land back on it; sources are swept by path and **hard** deleted, because nothing hangs off a source that a returning file would want back. Losing one of two copies therefore costs a source row and nothing else.

Ordering by a to-many edge makes ent group the query, so the sort column comes back unaggregated and Postgres rejects it. Query from the side that owns the column instead (see `items.ResumeItems`).

### Background work

Background work runs as Temporal workflows in a separate deployment. `gojellyfin worker` is the same image and the same binary as the server, run as a second deployment, and `TEMPORAL_HOSTPORT` is the server it dials; `TEMPORAL_NAMESPACE` defaults to `default`. With the address unset, background work is off and the server still starts, the way an empty `TRANSCODER_WORKERS` leaves transcoding off — a developer running the server alone gets a server rather than a dial error.

`internal/temporal` is the client and the worker; `internal/tasks` is the domain over them, and it is what the `ScheduledTasks` API is served from. A package declares what it runs beside the code that implements it: `scanner.Registration` names the workflow and its activities, and fx collects those into the worker through a value group, so the worker command lists no workflows of its own.

**The task id is the workflow id.** That is what gives a task singleton semantics without a lock or a dedupe column — Temporal refuses a second execution under an id that is already running, so pressing Start twice, `RefreshLibrary` and a schedule all collapse into one run. It also means state is one `DescribeWorkflowExecution` on a known id rather than a search over history, which is why `internal/tasks` needs no visibility query and no stored mapping.

The library scan fans its libraries out flat, because a library is a handful of rows. Per item work is what would need chunking into child workflows: a workflow has to replay its whole history, so a flat fan out of thousands of activities is the pattern Temporal's own guidance caps at a few hundred. Activities carry a bounded `RetryPolicy` — a library that cannot be read will not become readable by being asked again inside the same run — and heartbeat, so a scan that is merely slow is told apart from a worker that died.

One failing library does not abandon the others. The workflow collects every future and logs the ones that failed, because the sweep a failed library skipped is safe to leave until the next run while the ones that succeeded are not worth losing.

Triggers are not built. `UpdateTask` answers 501 rather than storing a schedule nobody reads; Temporal schedules are where they belong.

### Request identity

`internal/auth` owns identity on the context — it puts it on and takes it off, and nothing else knows the keys exist. `middleware.Auth` only parses what the client sent (`Authorization: MediaBrowser …`, `X-Emby-Token`, `?api_key=`) and calls `auth.Authenticate`, which resolves the token through `sessions` and returns a context carrying the session.

`auth.UserID` reads the session's user edge, so `sessions.ByToken` has to eager-load it; the foreign key is unexported and there is nothing to fall back on. Nothing else hangs off that query — an edge is eager-loaded when a caller reads it, not in anticipation of one, because `ByToken` runs on every authenticated request and each edge costs another round trip.

`auth.Authorization` carries the connection's `RemoteAddr` alongside what the client sent, because the strict handlers never see the `*http.Request` and `GetEndpointInfo` answers from the caller's address.

`ForgotPassword` answers `ContactAdmin` and `ForgotPasswordPin` refuses every pin, both without reading a request or touching the database. A pin is only as private as the channel that carries it, and the server has none — no mail, and the log and the pin file upstream writes are both read by whoever runs the box rather than by the account holder. Anything that issues a pin here hands account takeover to everyone who can read a shipped log. `gojellyfin resetpassword` is what `ContactAdmin` means: an operator with database access, reading the new password from stdin the way `adduser` does.

Handlers read `auth.UserID(ctx)`, `auth.SessionFrom(ctx)`, `auth.AuthorizationFrom(ctx)` and return `auth.ErrUnauthorized`; `middleware.TokenFrom` is the only thing left in the middleware package that handlers touch, and only because websocket and media URLs cannot send headers.

### Transcoding

Audio a client cannot direct play is encoded by ffmpeg **in the process serving the request**. There is no worker protocol and no pool: the pod that serves the stream is the pod that encodes it.

Which pods those are is a **routing decision**, not a protocol. `deploy/httproutes.yaml` sends `/Videos` and `/Audio` to `gojellyfin-transcode` and everything else to `gojellyfin`, and both deployments are the same image running the same `server` subcommand. A longer path prefix outranks a shorter one, so the split needs no ordering between the two routes.

That deletes the whole of what used to sit between them — the address list, the token, the round robin, the second subcommand and the HTTP hop that carried an encode from one pod to another. It also halves the bytes moved inside the cluster: a transcode used to cross the network twice, worker to API and API to client, and now crosses once.

`TRANSCODER_JOBS` bounds concurrent encodes on a pod and should not exceed its cpu limit, because one encode saturates about a core and a pod that accepts more than it can run makes every stream on it slower without finishing any sooner. Above that the pod answers 503 with a `Retry-After`, and **that refusal is the whole of the load balancing**: the gateway retries against another pod, with no pool, no load query and no shared state. `ErrNotAvailable` is the different answer — ffmpeg missing is not temporary, so it reaches the client as the 415 that says the device cannot play this.

The output is one progressive HTTP response rather than HLS, and that is what keeps the design small: a transcode is a single request from start to finish, so no second request has to find the same ffmpeg process, and nothing breaks with a second replica. HLS is what would force that question; the v12 spec carries no HLS path at all.

A client that goes away cancels the request context, which kills ffmpeg. A client that merely stalls blocks it on a full pipe instead, so a slow reader costs no CPU rather than racing ahead into a buffer. A client that vanishes *without* closing is the one case cancellation cannot reach: the write blocks rather than failing, and `TRANSCODER_STALL_TIMEOUT` bounds that to 30 seconds. What it must not kill is a client that is merely slow, and the two are told apart by where the relay is waiting — a slow client still takes a buffer's worth every so often, so the timer is reset by a read that moved something rather than by the pipe being full. Killing is both a cancel, which reaps ffmpeg, and a write deadline, which is the only thing that ends a write already blocked on a peer that stopped acknowledging.

ffmpeg answers a source it cannot read by exiting rather than by writing a short file, so `start` holds its return until the first byte of output arrives. A failure therefore reaches the handler as an error it can still answer 415 to, instead of as an empty stream the client believes.

Only containers ffmpeg can write to a non-seekable pipe are in the table, which rules out plain mp4; ogg is out for the second reason that it wants libvorbis, which not every ffmpeg build carries.

The tests run real ffmpeg and ffprobe what comes back, so CI installs ffmpeg and a green run means bytes a player can decode.

### Current state

Users, sessions, devices, libraries, items and their user data are real rows, as are playlists, their entries and their shares; most other handlers still return hardcoded data or a 501. Audio transcodes when a client cannot take the source; video is still direct play only (#481).

Live TV has no tuner (#525), no guide ingest (#526) and no recordings or scheduling (#527), and the `tuner_host`, `timer`, `series_timer` and `listings_provider` tables are declared but unread. Its read paths still answer an empty result rather than a 501, because a 501 makes the web client retry the section on a loop while an empty result is both true and renderable; every write path and every by-id lookup stays 501.

A fresh database has no way in through the API — `CreateUserByName` requires an administrator and nothing seeds one — so `gojellyfin adduser` creates the first one. It reads the password from stdin rather than a flag, which keeps it out of the shell history and the process list; that is a security property, not a convenience, so it stays true of any command that takes a password. One-off jobs that need the domain services rather than a running server belong beside it as another subcommand under `cmd/gojellyfin`.
