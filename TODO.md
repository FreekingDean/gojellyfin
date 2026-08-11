# TODO

Refactors and cleanups deferred out of a change. One line each.

- `config.ServerID` is a constant; it should come from stored server configuration.
- Nothing enforces the embedding depth that makes service dispatch work; a generated per-tag interface plus `RegisterXService` would turn the three silent failure modes into compile errors.
- The remaining stub handlers in `internal/server/handlers.go` (QuickConnect, SyncPlay) have no domain yet.
- `DisplayPreferences` keys its rows on the `user` and `item` edges, whose foreign keys are not declared as fields, so every read joins and the partial unique index has to name `item_display_preferences` as a string.
- Authorization is declarative in the spec (`security: [{CustomAuthentication: ["RequiresElevation"]}]`, 15 policies over 62+ operations). Generate the operation-to-policy map like `PublicOperations` and enforce it in the middleware rather than checking `IsAdministrator` per handler.
- Data-scoped authorization (blocked folders, parental ratings) belongs in the domain queries, not the edge.
- Sessions live in Postgres; they are short-lived and read on every request, so a cache like Redis is a better fit.
- `UserPolicy.max_parental_rating`/`max_parental_sub_rating` are rating-name enums, but the spec sends `MaxParentalRating` as an `int32` score and has no sub-rating field; the DTO leaves both unmapped until there is a rating-to-score table.
- `PostCapabilities` still 204s without writing; the columns it should fill (`playable_media_types`, `supported_commands`, `profile`, `supports_media_control`) now exist on `Device`.
- `atlas migrate diff` is commented out in `internal/store/generate.go` because it needs Docker; schema changes mean running it by hand.
- The scanner writes one `MediaSource` per item and replaces it wholesale on every probe, so nothing can hang off a source across scans yet (attachments, segments, trickplay).
- `Item.width`/`height`/`aspect_ratio` and the chapter, credit, genre and studio edges are modelled but nothing populates them.
- `GET /QuickConnect/Initiate`, `POST /Users/{userId}/Authenticate` and `POST /Users/{userId}/EasyPassword` are the only hidden Jellyfin routes without an alias; the reasons are in `unaliased` in `internal/http/http_test.go`, which fails if any other one is added upstream.
- Still 501 behind an alias: `UpdateUserItemRating`/`DeleteUserItemRating` (the `rating` and `likes` columns exist on `UserItemData`, so these are writable now), `GetGroupingOptions`, `GetSuggestions`, and the `GetUserImage`/`PostUserImage`/`DeleteUserImage` trio, which needs a user image on the model first.
- The `Videos` tag needs model work before it can be written: merged versions want a `primary_version_id` on `Item` that the scan upsert leaves alone (`MergeVersions`, `DeleteAlternateSources`), and additional parts want the scanner to group `part1`/`cd1` files under one item (`GetAdditionalPart`).
- `UpdateItemContentType` validates the content type against the collection types the server handles and then 204s without storing it; an item's content type is its library's `collection_type` today, and nothing models a per-folder override.
- `Item.forced_sort_name` is a bool but `BaseItemDto.ForcedSortName` is the sort name itself, so the metadata editor cannot write it; the genre, studio and people edges it also sends have no write path either.
- The scan upsert updates `name`, `sort_name`, `production_year`, `index_number` and `parent_index_number`, so the next scan overwrites what the metadata editor wrote; `lock_data` and `locked_fields` are written now but nothing reads them.
- `GetServerLogs` and `GetLogFile` stay 501: the server logs to stdout and there is no log directory to list, so they need a log sink on disk before they can answer anything real.
- `RestartApplication` and `ShutdownApplication` stay 501, and `GetSystemInfo` reports `CanSelfRestart: false`; stopping the process is a supervisor's job, and restarting it is not something a single process can do for itself.
- `GetWakeOnLanInfo` stays 501: waking the server is out of scope, so nothing owns the MAC addresses it would report.
- `auth.IsAdministrator` is the only admin check and nothing calls it yet; the ungated elevated operations (the `Environment` browser, `LibraryStructure`) want it until the generated policy map above replaces it.
- Tags move between Jellyfin releases: 10.10.0 has no `[Tags]` on `UserLibraryController`, so its operations carry the `UserLibrary` tag, while master retags the class `Library` and the favourite and rating actions `UserData`. Re-vendoring the spec past 10.10.0 moves those operations into different tag packages.
