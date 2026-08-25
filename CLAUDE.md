# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go reimplementation of a Jellyfin media server, serving the Jellyfin 12.0.0 HTTP API so that stock `jellyfin-web` and Jellyfin clients can talk to it.

## Rules

**Comments.** Don't write them, and there is no exception. A comment is almost always a sign the code is messy or doing too much — simplify the code instead of explaining it. Where an external constraint genuinely cannot live in a name, it goes in the External constraints section below rather than beside the code. `//go:` directives, `//nolint` and generated headers are not comments and stay.

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

Three things are shared between them, all in `main.go`: `withStore` opens the store, starts it and closes it around a callback, `readPassword` reads from stdin, and the root command's `PersistentPreRun` logs `system.Build()` — cobra runs it only for a subcommand that has work to do, so `--help` stays quiet while every deployed run says which build it is. The DSN does not live there — `internal/env` is the single source, which is why `migrate` can name the same database the server will use without repeating the string.

`make dev` and `make run` tee to `/tmp/gojellyfin.log`, so the log is on screen and readable by tooling at the same time.

`build` depends on `generate`, and `run` and `test` depend on `build`, so the generated code is never stale. The watch loop is the exception: `air` only builds, because regenerating re-emits ~95k lines on every save.

Run `air` through `tee` so the log is both on screen and readable at `/tmp/gojellyfin.log`; the request log is the fastest way to find what a client actually calls.

`air` owns `:8081` while it runs, so starting a second server alongside it fails with `ListenAndServe error: address already in use`. Check whether it is running with `pgrep -x air` (matching on a path fails — the process is just `air`), and the listener with `lsof -ti:8081 -sTCP:LISTEN` — without `-sTCP:LISTEN` it also matches browsers connected to the port, and killing those results is not what you want. An orphaned `.air/gojellyfin` can outlive its supervisor and keep serving stale code.

Requires a reachable Postgres. `DATABASE_URL` is required and the binary carries no default — a process that was not told which database to open fails at start rather than quietly dialing localhost. The development DSN (`postgres://localhost:5432/gojellyfin_development?sslmode=disable`) lives in the `Makefile` as a `?=`, so `make run`, `make dev` and `make test` supply it while an explicit `DATABASE_URL` still wins. Running `go test ./...` or the binary directly, outside `make`, means setting it yourself.

**`internal/env` is the only package that reads the environment.** It loads once into a `Config` that fx provides and every other package takes as a value, so the knobs are found by reading one struct rather than by grepping for `os.Getenv`, and a package under test is handed a value instead of having to set a variable. A one-shot subcommand calls `env.Load()` itself, because it has no lifecycle to hang it off.

The reading is `viper`, bound to the environment only — no config file, no flags, no watching. The `mapstructure` tag on each field is the variable's name, and `keys` walks `Config` to bind them, because `Unmarshal` only populates keys viper already knows about and `AutomaticEnv` does not register any. So a new variable is a new field and nothing else; `TestKeysCoverEveryVariable` names the whole set, which is where a rename shows up.

A malformed value is refused at start rather than ignored. `TRANSCODER_JOBS=lots` used to fall through to the core count and `TRANSCODER_STALL_TIMEOUT=30` to thirty seconds, so a typo in a manifest became a capacity problem with nothing to point at.

`HTTP_PORT` is what the server listens on and defaults to 8081, which is the port `air`, the `Dockerfile`, and every manifest in `deploy/` already name — the variable exists so a second process can be brought up beside them, not to move the default.

`serverModules` and `workerModules` in `cmd/gojellyfin` both list `env.Module`, and `TestWorkerModules` guards the second the way `TestServerModules` guards the first — a command that composes its graph inline has nothing to validate, so the worker starting without a config it needs is only found by running it.

`make test` needs one too — `internal/items` seeds real rows through `store.NewStore()` and fails rather than skipping when the database is unreachable, so a green run means the queries actually ran. Each test owns a library row and deletes it and its items on cleanup; point `DATABASE_URL` at a scratch database to keep development data out of it. CI runs the suite against a `postgres:16` service with `internal/store/migrations` applied by `atlas migrate apply`.

`make e2e` is the one test that boots the server. `cmd/gojellyfin/e2e_test.go` sits behind a `//go:build e2e` tag so `make test` never picks it up, creates a database of its own and drops it on the way out, runs `migrateCommand` and `addUserCommand` in process — piping the password through a swapped `os.Stdin`, because that command takes it no other way — seeds a library, and then starts the real `serverModules` graph and drives it over HTTP: public system info, a refused anonymous request, a refused wrong password, a login, the user behind the token, the library as a view, its items through the `/Users/{userId}/Items` alias, an item opened and favourited and re-read, and the websocket greeting. It takes about four seconds. `.github/workflows/e2e.yml` is its own workflow rather than a step in `ci.yml`, so it runs beside the unit tests instead of after them; it needs a postgres and the atlas CLI, but neither ffmpeg nor a pre-applied schema, because nothing here transcodes and the test migrates the database it made.

It picks its port by binding `:0` and reading the number back into `HTTP_PORT`, so it can never take `:8081` from a running `air`.

**There is no browser in this one.** `jellyfin-web` is not published anywhere a test can fetch it directly: no npm package, no built assets on its releases, and the only place the built client exists is inside the all-in-one `jellyfin/jellyfin` image (see the comment in `deploy/web-deployment.yaml`), which is 395MB and has to be unpacked to get at a directory of static files. So the smoke test is written against the API, which is the contract the client talks to. A real integration test that boots the client on top of it is wanted and tracked in #558 — that is a second test rather than a change to this one, because what the two catch is different: this one fails when the server stops serving, and that one fails when the client stops being able to use what it serves.

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

`TestServerModules` in `cmd/gojellyfin/server_test.go` runs `fx.ValidateApp` over `serverModules`. A forgotten `Module` compiles cleanly — nothing references one except the list it belongs to — so the build cannot catch it and the server dies on start instead. `ValidateApp` checks the graph without opening a database or binding a port; dropping `mediainfo.Module` fails it with `missing type: *mediainfo.Server`.

### API layer: generated, with an unimplemented base

`internal/server/api` is entirely generated and should never be hand-edited. `go:generate` runs two steps:

1. `oapi-codegen` (strict server + models + embedded spec) → `jellyfinapi.gen.go`, a ~95k-line file defining `StrictServerInterface` with one method per Jellyfin endpoint.
2. `./gen` (a local AST tool) reads that interface and emits `unimplemented.gen.go` — an `Unimplemented` struct implementing every method with `ErrNotImplemented`.

`server.Server` embeds `api.Unimplemented`, so it satisfies the full interface while only the endpoints actually written exist as real methods. **Implementing an endpoint means adding a method to `internal/server` that shadows the embedded stub.** `ErrNotImplemented` is mapped to HTTP 501 by the response error handler in `internal/http/http.go`.

`spec/jellyfin-openapi-stable.json` is the vendored upstream spec and `spec/overrides.json` the `x-go-type` overrides `specpatch` applies to it where upstream types generate badly, so re-vendoring means replacing one file rather than re-applying patches by hand.

**Two versions, and they are different facts.** `system.JellyfinVersion` is the API this server speaks and `gen` writes it into `internal/system/jellyfinversion.gen.go` from the spec's `info.version`, because a second copy maintained by hand is how it came to claim 10.10.0 while the spec said 12.0.0. `internal/system` may not import `api`, which is why the generator writes across rather than the domain reading `GetSwagger()`. `system.Build()` is the other fact — which build of gojellyfin is running — stamped by `-ldflags -X` from the `Makefile` and the `Dockerfile`, defaulting to `dev` for a plain `go build` since `.dockerignore` drops `.git`. It reaches clients as `SystemInfo.PackageName`, not as `Version`: `jellyfin-web` refuses a server whose `Version` is below the `@jellyfin/sdk` minimum of `10.9.0` (`ServerUpdateNeeded`) and its discovery scores a `ProductName` other than `Jellyfin Server` as unusable, so neither field is ours to spend. Every subcommand logs `Build()` before it runs.

### Request path

`internal/http` owns the `net/http` server and composes two middleware layers: `middleware.HttpMiddleware` (stdlib handler wrappers — CORS, logging) applied outside, and `api.StrictMiddlewareFunc` (operation-aware, has the operation ID) applied inside the generated handler.

Routing uses a hand-rolled `internal/http/mux`, not `http.ServeMux`, because Jellyfin clients hit case-insensitive paths and paths with literal dots (`stream.mp4`). Patterns compile to regexes: `{name}` captures a segment, `*` is a catch-all, matching is case-insensitive, and a trailing slash is always tolerated. Path params land via `r.SetPathValue`.

`GET /socket` is registered outside the generated API for the websocket keepalive loop (`internal/server/socket`).

So are routes Jellyfin serves but hides from its own OpenAPI document with `[ApiExplorerSettings(IgnoreApi = true)]`. There are 36 of them, almost all the pre-10.9 `/Users/{userId}/…` spellings, and no version of the spec contains any of them — searching a newer spec will not find them either. `spec/jellyfin-hidden-routes-10.10.0.txt` is the extracted list, and `TestLegacyRoutes` fails if one of them has neither an alias nor a stated reason. It is still the 10.10.0 extraction while the spec is 12.0.0, and the two have drifted (#589). `jellyfin-web` still calls some, so the symptom is a 404 for a path the spec does not define; the definition is in the controller source (`GET /Users/{userId}/Items/{itemId}` is `Jellyfin.Api/Controllers/UserLibraryController.cs`).

`legacyRoutes` in `internal/http/http.go` maps each to its documented spelling and re-dispatches through the mux, so an alias costs a table entry rather than a handler. `[Obsolete]` alone does **not** mean missing — most obsolete routes are still documented and already generated; only `IgnoreApi` hides them.

They register after the generated routes so a documented literal wins any overlap, and most-specific first: the mux matches in registration order with no notion of specificity, so `/Users/{userId}/Items/{itemId}` registered before `/Users/{userId}/Items/Resume` swallows it and passes "Resume" as an item id. `legacyPatterns` sorts by literal segments to keep that from depending on map order.

### External constraints

Code carries no comments, so the third-party behaviour it is written around is written down here instead. Each of these cost time to find once and would cost it again.

**Clients.** Jellyfin clients send query parameters in PascalCase while the spec declares them camelCase, and send `Param=` with an empty value where a value is undefined, which fails to parse as an int or a uuid and answers 400; `middleware.HttpCanonicalQuery` maps the case back and drops the empty, and `queryparams.gen.go` is the table it maps against. Media URLs, websocket handshakes and HLS subtitle playlists cannot carry an `Authorization` header, so the token rides in the query — `jellyfin-web` sends `ApiKey` and older Emby clients send `api_key`, and neither casing is canonicalised because those routes register outside the generated API. Clients percent-encode the `Authorization` header's values, so `Jellyfin%20Web` arrives verbatim. The websocket client treats an unanswered `KeepAlive` as a dead socket and reconnects, so the timeout is advertised as 60s and `ForceKeepAlive` is pushed at half that. Clients switch to `SystemInfo.LocalAddress` when they believe they are on the same network, so an address this server cannot confirm is worse than none: they stop talking to the address that reached them and never come back. `jellyfin-web` retries a playback error until the url refuses both stream copies, which is why `mediaSourceDto` writes `SupportsDirectPlay` and `SupportsDirectStream` false even though nothing here reads them.

**Generated code.** `oapi-codegen` normalizes operation ids into Go method names, so the spec's `GetBrandingCss_2` becomes `GetBrandingCss2` and the middleware only ever sees the normalized form. It splits a JSON body across two fields depending on content type, which is what `apiutil.Body` folds back together. It types the vendored spec's free-form `DeviceProfile` as an opaque object, so `mediainfo` reads it back out of json rather than binding a generated type. Body decoding and parameter binding both answer 400 from inside the generated wrapper, so the handlers in `internal/http/http.go` are the only place either reason is visible.

**Authorization.** Scopes come from the spec through `api.OperationPolicies`, so an operation is authorized by what upstream declares rather than by whether a handler remembered to check. An operation in neither that table nor `PublicOperations` is refused, and `users.Satisfies` refuses a scope it does not name even for an administrator, so a re-vendored spec that renames one fails loudly and closed rather than serving it unguarded.

**ffmpeg and ffprobe.** `image.DecodeConfig` needs each format's decoder registered by blank import or every image reports 0x0. ffprobe reports muxer families like `matroska,webm`, so the file extension picks the one clients are told about, and some containers hang their tags off the media stream rather than the format, where the format wins a shared key. The ffprobe timeout is deliberately shorter than the heartbeat window a step is given: an abandoned step is never told to stop, and a read that has wedged has no loop to be told in. ffprobe's JSON writes every numeric as a string and omits a key it has no value for rather than writing `"N/A"`, so `,string` is safe on `bit_rate` and `sample_rate` but wrong on anything that is not an integer: `duration` is `"1.023000"` and `avg_frame_rate` is the rational `"30/1"`. `,string` is strict, so one `ParseInt` failure fails the whole decode and every file stops probing — which is why the fields are typed to what the store holds and `internal/ffmpeg` probes a real file rather than trusting the build. ffprobe missing is not a startup failure: `ffmpeg.New` records an empty path and `ProbeFile` answers `ErrNotAvailable`, which the scan reads as nothing to learn, so a box without it serves rather than refusing to build the graph — the same answer `internal/transcode` gives a missing ffmpeg.

**Postgres and ent.** The `library_id,key` unique index is deliberately not partial on `deleted_at`, because a title that comes back has to conflict with the row it left behind. External subtitle streams are indexed off the end of the container's own, so the source row is locked for the whole rewrite or two writers allocate the same index. Next-up needs `DISTINCT ON` to pick one row per series with the `ORDER BY` deciding which, which the query builder cannot express, so `items.NextUpEpisodes` is raw SQL. Postgres does not parameterise a database name, so the e2e fixture interpolates one it built itself. **`ORDER BY kind` is not a batch order.** The column stores the enum by name and ent generates the constants alphabetically, so sorting on it hands back Episode, Movie, Season, Series — episodes first, which is exactly backwards for anything that has to identify a parent before its children. `items.ItemsNeedingMetadata` therefore ranks the rows with a `CASE` built from the order its `kinds` argument was passed in, and `metadata.identifiable` is what passes them parent first: a season resolves its series through the series' provider ids and an episode through the season's, so a parent identified after its child leaves that child a miss until the next run. Ties break on id rather than on a column the run writes, so the list does not reshuffle underneath the caller walking it. Simplifying the `CASE` back to the column reintroduces the bug and `internal/items/metadata_test.go` is what catches it.

### Tracing

Every operation gets one span, named for its **operation id**. That is why the span sits on the `api.StrictMiddlewareFunc` layer rather than the stdlib one: only the inner layer is handed the operation id, and a span named for a route pattern or a raw path is much less useful. It is last in `apiMiddleware`, which the generated wrapper folds outermost, so it measures authentication and authorization too.

`OTEL_EXPORTER_OTLP_ENDPOINT` is the only spelling read. The OTLP specification also defines a signal-specific `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`, and supporting both would mean carrying its precedence rule and its different path semantics for a second way to say the same thing; one prescribed variable is the standing decision. Unset means tracing is off: no provider, a noop tracer, nothing dialed, and `tracing.Enabled` false, which is what keeps the span middleware out of the stack entirely rather than wrapping every request in a tracer that discards it.

The endpoint is checked in `tracing.New` rather than in `env.validate`, because what makes an OTLP endpoint valid is the exporter's business and `env` only knows it wants a string. The exporter answers a URL it cannot parse by logging and carrying on against its own localhost default, so a malformed one is refused at start instead.

**`internal/observability/tracing` is the only package that imports otel.** A caller gets `StartRequest` and a `Span` with `End` and `Fail`, not a `trace.Tracer`, so the middleware names no otel type and the backend stays swappable; `Recorded` is the same seam for tests, handing back a `Recorder` that answers in span names and attribute values. It is composed through `observability.Module`, which `server` and `worker` both list, and it registers `OnStop` only — a provider is exporting from the moment it is built, so there is nothing to start. The flush is bounded on the way out, because a collector that has gone away must not hold the process open.

**Streaming endpoints are excluded.** `streams` in `internal/http/middleware/oapitracing.go` names `Videos` and `Audio`, the same two roots `deploy/httproutes.yaml` splits on, matched case-insensitively because the mux is. A progressive response runs for the length of the media, so a span covering one is open for hours — a leak rather than a trace. `GET /socket` needs no entry: it is registered outside the generated API, so it never reaches this layer. The streaming case in `TestOapiTracing_Middleware` is what keeps the exclusion from being incidental.

Nothing the client sent goes on a span. The query string carries `api_key` and some paths carry names, so only the method and the operation id are recorded; `TestOapiTracing_Middleware` holds that, because traces ship to third-party backends.

### Domain services

Three layers, split on what each is allowed to know:

- **`internal/{auth,sessions,users,items,libraries,tasks,config}` — domains.** Behaviour over the ent models, exposing a `Service` built with `New(client *store.Client)`. **A domain package must never import `internal/server/api` or `internal/http/middleware`.** That invariant is what the layout rests on; check it with `grep -rl 'server/api\|http/middleware' internal/<domain>/`, which must come back empty. `auth` owns hashing, tokens and request identity; `sessions` owns the active session and device rows, which are Jellyfin sessions rather than login state; `tasks` owns the workflows the dashboard drives.
- **`internal/server/<tag>` — one package per spec tag.** Named for the tag (`userlibrary`, `librarystructure`, `mediainfo`), holding exactly the operations that tag declares and a `Server` with the domain services it needs. Add a tag package by looking the operation up in the spec, not by guessing where it feels like it belongs — `AuthenticateUserByName` is a `User` operation, `GetBitrateTestBytes` is `MediaInfo`.
- **`internal/server/apiutil`** — four generic helpers (`Ptr`, `Deref`, `OrElse`, `Body`) and nothing else. It imports none of our packages, and no domain knowledge belongs in it; Go cannot alias generic functions, which is the only reason they are shared rather than copied.

**A tag package never imports another tag package.** Translation two or more tags need lives in **`internal/server/dto`** — `ItemDto`/`ItemDtos`/`LibraryView`/`UserItemDataDto`/`Kinds`/`SessionDto`/`CultureDtos`/`CountryInfos`/`ParentalRatings` today, in both directions, model→DTO and DTO→model. It may import `api`, `apiutil` and the domains, and it must never import a tag package. Domains cannot hold any of it, because they would have to import `api`.

Translation a single tag uses stays in that tag's `dto.go` and is **unexported** — `user.userDto`, `playlists.playlistDto`, `scheduledtasks.taskInfo`. That is what gives the rule its teeth: **no exported translation function in a tag package**, so there is nothing for a second tag to reach for, and the second caller is the moment a helper moves to `dto`. `Test` in `internal/server/translation_test.go` parses every tag package and fails on an exported function whose signature names an `api` type. Its exception table is the whole of the escape hatch: `configuration.ServerConfiguration`/`BrandingConfiguration` read stored config rather than translate. Adding a name there needs a reason, and a stale entry fails the test too.

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

**An `Item` is a title; a `MediaSource` is a file.** The item carries no path — its identity is `key`, unique with `library_id`, derived by the scanner from the name and year (`movie:the-matrix:1999`, `series:the-wire`, `season:the-wire:1`, `episode:the-wire:1:3`). Keying on location meant a moved file changed identity and a reorganised library lost every resume position and play count; a name-derived key survives the move. The key is internal — no endpoint takes one in its path and the spec has no field to put one in, so clients only ever see the UUID. `MediaSource` owns the path, unique per library, and an item may have many, which is how a 4K and a 1080p rip of one film become one entry with two versions.

The key is the whole of the grouping: two files whose names parse the same are two copies of one title, and a cut that is named differently on disk already parses to a key of its own. Nothing reads the runtime to second-guess that, so the scan needs no ordering between probing a file and deciding which item it belongs to.

`probed_at` and `date_modified` live on the source, so an unchanged file costs no ffprobe.

**Upgrading an install that predates the key is `migrate` plus a scan, and nothing else.** The migration stands the old path in as the key so no row is left keyless, and the scan rewrites it: `scanner.rekeyLegacy` derives the real key from columns the row already carries and updates in place, so the walk upserts onto the existing row rather than creating a sibling and every item id survives. That is what keeps watch state, favourites, resume positions and playlist entries attached — they reference items by id, and an upgrade that re-created rows would orphan all of them silently. It runs inside `scanLibrary` because it has to precede the sweep, which deletes by key; a one-shot subcommand would lose the race with a scheduled scan.

Two old rows that derive one key are the duplicate the key exists to collapse, so `items.Merge` folds the second into the first rather than refusing: its files, its children and its playlist entries move over, and a user with data on both rows keeps whichever answer says the title was watched — played and favourite carry, the count and the position take the further of the two. Refusing would wedge the library instead of losing one row: the collision is on a unique index, so every later scan fails at the same item and that library never scans again.

An item with more than one source has to pick one, and the two callers want opposite things. A stream takes `items.PreferredSource`, which is **compatibility over quality** — the best encode a client cannot decode is worse than the worse one it can — and falls back to the richest source when nothing matches, so the caller can still remux it or refuse with a status rather than handle a nil. A download takes `items.BestSource`, ignoring compatibility entirely, because the bytes are being saved rather than decoded and what the far end can play is its own problem. Matching is on container and audio codec only; a kind the client said nothing about is unrestricted rather than refused, so only a browser — which declares nothing — is held to the tables in `internal/server/stream`. Video codec is deliberately not part of it (#532), which is consistent with video being direct play only (#481).

Columns owned by a background process must be left out of an upsert's `DoUpdates` — the scan clobbering probe-owned columns like `run_time_ticks` was a real bug. `SaveScanned` writes the item's `date_modified` and `SaveSource` the source's; the probe writes `container`, `run_time_ticks`, `size`, `bitrate`, `probed_at`, `tags`, the source's streams and the genre, studio and credit edges, and rewrites all of them on every run, so the edges are cleared and re-added rather than merged.

The sweep is split the same way. Items are swept by key and **soft** deleted, because the row carries the id the watch state hangs off and a returning title has to land back on it; sources are swept by path and **hard** deleted, because nothing hangs off a source that a returning file would want back. Losing one of two copies therefore costs a source row and nothing else.

Ordering by a to-many edge makes ent group the query, so the sort column comes back unaggregated and Postgres rejects it. Query from the side that owns the column instead (see `items.ResumeItems`).

### Background work

Background work runs as Temporal workflows in a separate deployment. `gojellyfin worker` is the same image and the same binary as the server, run as a second deployment, and `TEMPORAL_HOSTPORT` is the server it dials. Both reach `internal/jobs` as `env.Config.Temporal` rather than through `os.Getenv`. With the address unset, background work is off and the server still starts — a developer running the server alone gets a server rather than a dial error.

`TEMPORAL_NAMESPACE` has no default, and `jobs.NewClient` refuses to build a client without one rather than `env` inventing it: a namespace is only required once there is a server to dial, and defaulting it silently puts every run in whichever namespace the constant happened to name. The check belongs with the client because that is what needs the value.

`internal/jobs` is the client, the worker and the registry the `ScheduledTasks` API is served from, and it is the only package that names the engine — a job is written against `jobs.Job`, `Context`, `Step` and `Child`, which `internal/jobs/abstraction_test.go` enforces. A package declares what it runs beside the code that implements it: a `Job` names its steps and its children, and `internal/scanner/fx.go` registers it, so the worker command lists no workflows of its own.

**The task id is the workflow id.** That is what gives a task singleton semantics without a lock or a dedupe column — Temporal refuses a second execution under an id that is already running, so pressing Start twice, `RefreshLibrary` and a schedule all collapse into one run. It also means state is one `DescribeWorkflowExecution` on a known id rather than a search over history, which is why `internal/tasks` needs no visibility query and no stored mapping.

The library scan is a walk and a fan out, split because they are different sizes. `ScanLibrary` carries legacy keys forward, walks one library, writes its titles and its files and sweeps what has gone, and there is one of those per library, so they fan out flat. A library it could not finish fails the step, which is what keeps a rekey that could not run from being followed by a walk that would create siblings — and an unprobed library waits for the next run rather than being probed against a structure nobody wrote. The probes are per file and there are thousands, so they fan out through `ProbeChunk` children of a hundred each: a workflow replays its whole history, and a flat fan out of thousands of activities is what Temporal's own guidance caps at a few hundred. The parent's history holds a handful of children, and each child's holds its own hundred probes.

**A child's work is selected from the rows, not handed down by the walk.** `items.SourcesNeedingProbe` is the predicate — no `probed_at`, or one older than the file's `date_modified` — so a run that died leaves the next one able to derive what is outstanding rather than replay what the walk happened to touch, and the same predicate is what a progress denominator would count. A file whose row went away between the selection and the probe is done rather than failed.

`jobs.Child` scopes a child's id to the parent's run, so a chunk cannot take the name of a chunk the run before it has not finished tearing down, and two chunks of one run cannot collide. The parent is still a singleton under its own name, which is what keeps a second Start from starting a second fan out.

Activities carry a bounded `RetryPolicy` — a library that cannot be read will not become readable by being asked again inside the same run — and heartbeat, so work that is merely slow is told apart from a worker that died. The heartbeat has to be inside the loop: `ScanLibrary` beats per file it walks and `ProbeSource` per file it reads, because one beat at the start of a walk that takes an hour is what a `TIMEOUT_TYPE_HEARTBEAT` on a large library looks like. A probe saturates about a core, so a chunk runs its files one at a time and the worker caps concurrent activities at `GOMAXPROCS` — the parallelism worth having is across chunks.

Nothing at any level abandons its siblings. The workflow collects every future and logs the ones that failed: the sweep a failed library skipped is safe to leave until the next run, and a file ffprobe cannot read is one file rather than the chunk it landed in. A library that could not be walked is not probed either, because there is no structure to hang the probe off.

Triggers are not built. `UpdateTask` answers 501 rather than storing a schedule nobody reads; Temporal schedules are where they belong.

### Metadata providers

**`internal/metadata` is what anything else names; `internal/metadata/tmdb` is one implementation of it.** It sits under `metadata` because nothing outside `metadata` calls it, and the tree should say so. The interface is declared by the consumer, in `metadata/provider.go`, and `metadata.Module` aggregates `tmdb.Module` the way `server.Module` aggregates its tag packages — so `server.go` and `worker.go` list `metadata.Module` and learn nothing about who answers. The scheduled task the dashboard shows says it identifies items, not who it asks.

That split is a rule, not a preference: **no package outside `internal/metadata` may import the provider.** Check with `grep -rln 'metadata/tmdb' --include='*.go' . | grep -v '^./internal/metadata/tmdb/'`, which must return `internal/metadata/fx.go` and nothing else.

The rule is about imports, not about the letters T-M-D-B. `internal/env` carries a `TMDB` struct holding `TMDB_API_KEY`, and that is correct rather than a leak: `env` is the one place every variable this binary reads is written down, and the variable is named for the service whose key it is because that is what an operator puts in a manifest. A provider-neutral name would make the manifest lie and would break the moment a second provider needs a key of its own. `env` imports none of our packages, so naming a variable costs no coupling — the leak to avoid is a package reaching for the provider, and `metadata` still takes only the `Provider` interface.

`metadata` owns the job, the batch loop and the locks. `tmdb` owns the calls and the translation from TMDB's payloads into `items.Metadata` — including from TMDB's vocabulary into ours, which is why `Status` is mapped to Jellyfin's `Continuing`/`Ended`/`Unreleased` rather than passed through, and why a movie writes no series status at all.

The calls go through `github.com/cyruzin/golang-tmdb`, which models every payload this needs. Two of its behaviours are deliberately not used. Its `SetClientAutoRetry` retries a 429 in an unbounded loop and never retries a 5xx, so a `RoundTripper` in `retry.go` does the backoff with a bound; and every request it builds carries its own `context.Background`, so a cancelled run cannot interrupt one in flight — the limiter before each call is where cancellation lands, which bounds a cancelled step to a single request's timeout.

Two things keep the dependency pointing one way. A miss is a `false` return rather than a sentinel error, so the implementation never imports its consumer for one variable; and `Season` and `Episode` take the series' whole `ProviderIds` map, so each provider reads its own key out and no key name escapes the package that owns it.

There is **one** provider and one binding — no fx value group and no priority order until a second provider exists to need them. The seam is the interface.

It is bring your own key: `TMDB_API_KEY` reaches the client as `env.Config.TMDB.APIKey` rather than through `os.Getenv`, and unset leaves the provider disabled and the job a no-op, so a developer running the server alone still gets a server. An absent key is deliberately not a validation failure — `env` refuses a malformed value, not a missing optional one. We embed no key of our own: there is then none to share, none to throttle and no attribution owed for one.

The job runs on its own rather than inside the scan, and does **not** fan out per item. The work is IO bound on a rate limited API, so a step per item multiplies the request rate and finishes no sooner; one step loops over at most `batchSize` items, spaced by the client's own limiter and heartbeating between them. What a run leaves, the next run picks up.

The batch is derived from the rows — `items.UnidentifiedItems` asks for items whose `provider_ids` is null — rather than handed over, so a crash re-asks the question instead of replaying a stale list. Writing those ids well is the point: item identity is otherwise name and year, which collides when two films share both.

A season and an episode are looked up through their series' ids rather than by name, so the series has to be identified first. `identifiable` is written parent first and the batch is sorted by position in it, which is what makes one run enough — the rows come back oldest first, and a series edited since its episodes were scanned would otherwise be reached after them and leave the whole show for the next run. Both walk `items.Ancestors` for the `Series`, so a season that matched nothing costs its episodes nothing. Specials are season zero on both sides and need no case of their own.

Nothing fetches artwork. A provider's poster is a URL, and an `Image` row is a path to a file the scan found beside the media, so storing one means downloading and caching it — that is #576, not this.

`lock_data` and `locked_fields` are Jellyfin's semantics and `metadata` honours both. `LockData` keeps an item out of the batch entirely; a field named in `LockedFields` is dropped from what is written, by `stripLockedFields`, which takes a pointer because it edits what it is handed and hiding that would hide an overwrite later. The provider ids themselves survive a field lock, because they are identity rather than metadata and Jellyfin has no lock for them.

Two requests per item, not more: the detail call carries `append_to_response=release_dates` (or `content_ratings,external_ids`), so the certification and the IMDb id arrive with the record instead of costing a third round trip. A 429 or a 5xx backs off and retries rather than failing the run; a 404 or an empty search is a miss, which is not an error because the title may be one TMDB gains later.

The two packages test at their own seam: `metadata` runs the loop and the locks against a stub provider and a real database, and `tmdb` points its client at an `httptest.Server`. A green run needs no key and CI never calls TMDB.

### Request identity

`internal/auth` owns identity on the context — it puts it on and takes it off, and nothing else knows the keys exist. `middleware.Auth` only parses what the client sent (`Authorization: MediaBrowser …`, `X-Emby-Token`, `?api_key=`) and calls `auth.Authenticate`, which resolves the token through `sessions` and returns a context carrying the session.

`auth.UserID` reads the session's user edge, so `sessions.ByToken` has to eager-load it; the foreign key is unexported and there is nothing to fall back on. Nothing else hangs off that query — an edge is eager-loaded when a caller reads it, not in anticipation of one, because `ByToken` runs on every authenticated request and each edge costs another round trip.

`auth.Authorization` carries the connection's `RemoteAddr` alongside what the client sent, because the strict handlers never see the `*http.Request` and `GetEndpointInfo` answers from the caller's address.

`ForgotPassword` answers `ContactAdmin` and `ForgotPasswordPin` refuses every pin, both without reading a request or touching the database. A pin is only as private as the channel that carries it, and the server has none — no mail, and the log and the pin file upstream writes are both read by whoever runs the box rather than by the account holder. Anything that issues a pin here hands account takeover to everyone who can read a shipped log. `gojellyfin resetpassword` is what `ContactAdmin` means: an operator with database access, reading the new password from stdin the way `adduser` does.

Handlers read `auth.UserID(ctx)`, `auth.SessionFrom(ctx)`, `auth.AuthorizationFrom(ctx)` and return `auth.ErrUnauthorized`; `middleware.TokenFrom` is the only thing left in the middleware package that handlers touch, and only because websocket and media URLs cannot send headers.

### Transcoding

Audio a client cannot direct play is encoded by ffmpeg **in the process serving the request**. There is no worker protocol and no pool: the pod that serves the stream is the pod that encodes it.

**A client is handed one source and one url, and neither is its choice.** `PlaybackInfo` answers with exactly one `MediaSourceInfo` — `SupportsDirectPlay` and `SupportsDirectStream` false, `SupportsTranscoding` true, and a `TranscodingUrl` this server generated — or with no sources at all and `ErrorCode: NoCompatibleStream`. The client never learns a second file exists, which is why the version picker in `jellyfin-web` has nothing to pick from: an item's 4K and 1080p copies are one answer now, not two.

**Which file that is belongs to `internal/items`.** `items.SourceFor(ctx, itemID, items.Capabilities)` is the whole question — of the files behind this item, which one, and what has to change about it — and `items.Plan` is the answer. It walks the files tallest first and returns the first whose plan can be served; there is no ranking, because a file's plan is already the least that has to change about it. That is Dean's ordering with none of the machinery: a 4K needing its audio converted is returned before a 1080p that plays untouched because the 4K is reached first, and a file needing a picture encode is passed over. At one resolution `richer` decides, which is bitrate then size, so the cheaper plan does not win a tie.

`internal/server/mediainfo` does three things and nothing else: `capabilities` reads the posted `DeviceProfile` into `items.Capabilities`, `SourceFor` is called, and `mediaSourceDto`/`served`/`streamURL` translate the plan back. The domain cannot see `api`, so a device profile becomes a domain type at the boundary.

A client says what it decodes in two places and both are read. A `DirectPlayProfile` is a triple — container, video codec, audio codec — and the `CodecProfiles` beside it carry the conditions: Chrome names `h264` and then says it wants `SDR`, a `VideoProfile` it knows, a `VideoLevel` it can reach, and neither interlacing nor anamorphic pixels. Reading only the first was the AC-3 bug again in the picture, since an HDR remux passes the codec check and plays washed out with nothing to say so. Which of those fails decides the cost, and `items.Capabilities.plan` answers it in one pass: the file as it is, the container changed with both streams copied, the audio converted with the picture still copied, or a picture encode — which is #481, so `Change.Available()` is false for it and the file is passed over.

**A condition is held as hard.** Every one jellyfin-web writes carries `IsRequired: false`, and the client never evaluates them itself, so what that flag means for direct play cannot be read off the bundle — upstream uses it to decide what a transcode must enforce, not what direct play may ignore. Only the three verbs the client writes for video are implemented (`EqualsAny`, `NotEquals`, `LessThanEqual`); an unread verb, an unread property and an unprobed value all pass, because a file is never refused for something nobody read. That works because `scanner.probe` now fills in `video_range_type`, `is_interlaced` and `is_anamorphic`, which the ent schema had carried unwritten since it was generated — a file whose transfer characteristic ffprobe does not report is left unknown rather than called SDR.

**The url is an instruction, not a hint.** It names the container and the audio codec the response will carry, and `stream.Serve` executes it: the same container as the source means the file, a different one means ffmpeg, and the audio is copied when the url names the codec the source already has. Nothing in the handler re-derives the decision, which is what deleted the browser-guessing tables and `items.PreferredSource` along with them — the url names its source id, so there is nothing to prefer.

Codec names alone cannot tell a copy from an encode — aac re-encoded to aac is still aac — so the tests fingerprint each stream with ffmpeg's md5 muxer and compare it against the source. A needless audio encode still plays, so nothing but the cpu would notice it.

**Codec names are compared as written, because both sides write them the same way.** `jellyfin-web` builds its `DirectPlayProfiles` out of `h264`, `hevc`, `mpeg2video`, `vc1`, `msmpeg4v2`, `vp8`, `vp9`, `av1`, `aac`, `mp3`, `ac3`, `eac3`, `mp2`, `pcm_s16le`, `pcm_s24le`, `truehd`, `aac_latm`, `opus`, `flac`, `alac` and `vorbis` — ffprobe's own spellings, which is what `scanner.probe` stores verbatim. The one codec with two names in circulation is DTS, and the client sends `dca` and `dts` together rather than choosing. An alias table was written and thrown away: nothing in it would ever have fired, and a guessed alias is a new way to be wrong.

The decision lives in the url rather than on the session because the capability data only ever arrives on the `PlaybackInfo` body: `jellyfin-web` posts no `DeviceProfile` with `Sessions/Capabilities/Full` (#579).

Three things have to agree and are tested together: the suffix in the url, the `container` parameter, and the `Content-Type` of the response. A browser dispatches on the header, which is how an AC-3 rip played its picture in silence.

`UserPolicy.EnableVideoPlaybackTranscoding` is answered false: the picture is copied and never re-encoded (#481), and that policy is what `jellyfin-web` reads to decide whether to offer a bitrate menu. `MaxStreamingBitrate` is not honoured, so the menu would change nothing (#580).

**Seeking is by position rather than by byte.** The client believes `PlayMethod: Transcode`, so a seek it cannot satisfy from what it has buffered re-posts `PlaybackInfo` with `StartTimeTicks` and follows the url that comes back, and ticks map straight onto ffmpeg's `-ss`. A source served untouched is a file, so `http.ServeContent` answers byte ranges and the client seeks in place instead. A remux copies the picture, so it starts at the keyframe at or before the position asked for.

`UserPolicy.EnableVideoPlaybackTranscoding` is answered false for the same reason: the picture is copied and never re-encoded (#481), and that policy is what `jellyfin-web` reads to decide whether to offer a bitrate menu. `MaxStreamingBitrate` is not honoured, so the menu would change nothing (#580).

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
