# TODO

Refactors and cleanups deferred out of a change. One line each.

- `AuthenticationProviderId` is hardcoded to `"authentik"` in `defaultPolicy`.
- `config.ServerID` is a constant; it should come from stored server configuration.
- Nothing enforces the embedding depth that makes service dispatch work; a generated per-tag interface plus `RegisterXService` would turn the three silent failure modes into compile errors.
- The remaining stub handlers in `internal/server/handlers.go` (DisplayPreferences, QuickConnect, SyncPlay, bitrate test) have no domain yet.
- Authorization is declarative in the spec (`security: [{CustomAuthentication: ["RequiresElevation"]}]`, 15 policies over 62+ operations). Generate the operation-to-policy map like `PublicOperations` and enforce it in the middleware rather than checking `IsAdministrator` per handler.
- Data-scoped authorization (blocked folders, parental ratings) belongs in the domain queries, not the edge.
- Sessions live in Postgres; they are short-lived and read on every request, so a cache like Redis is a better fit.
