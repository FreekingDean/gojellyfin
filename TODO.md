# TODO

Refactors and cleanups deferred out of a change. One line each.

- `config.ServerID` is a constant; it should come from stored server configuration.
- Nothing enforces the embedding depth that makes service dispatch work; a generated per-tag interface plus `RegisterXService` would turn the three silent failure modes into compile errors.
- The remaining stub handlers in `internal/server/handlers.go` (DisplayPreferences, QuickConnect, SyncPlay, bitrate test) have no domain yet.
- Authorization is declarative in the spec (`security: [{CustomAuthentication: ["RequiresElevation"]}]`, 15 policies over 62+ operations). Generate the operation-to-policy map like `PublicOperations` and enforce it in the middleware rather than checking `IsAdministrator` per handler.
- Data-scoped authorization (blocked folders, parental ratings) belongs in the domain queries, not the edge.
- Sessions live in Postgres; they are short-lived and read on every request, so a cache like Redis is a better fit.
- `UserPolicy.max_parental_rating`/`max_parental_sub_rating` are rating-name enums, but the spec sends `MaxParentalRating` as an `int32` score and has no sub-rating field; the DTO leaves both unmapped until there is a rating-to-score table.
- `PostCapabilities` still 204s without writing; the columns it should fill (`playable_media_types`, `supported_commands`, `profile`, `supports_media_control`) now exist on `Device`.
- `items` and `libraries` are still on gorm and nothing provides `*gorm.DB`, so the fx graph does not resolve. Migrating them needs schema decisions first: `Item` has no `size`/`bitrate`/`probed_at`/`date_modified`; `MediaStream` hangs off `MediaSource` but the scanner writes streams per item; `Image` has `blur_hash` but no `tag`; `LibraryPath` rows became `Library.locations`; `LibraryOptions` has ~40 required fields with no defaults.
- `atlas migrate diff` is commented out in `internal/store/generate.go`, so `migrations/*.sql` is behind `migrate/schema.go`.
