# TODO

Refactors and cleanups deferred out of a change. One line each.

- `ptr`/`deref`/`orElse`/`body` are copied into each domain package; Go can't alias generic functions, so sharing them means qualified calls at every site. Revisit if the list grows.
- `AuthenticationProviderId` is hardcoded to `"authentik"` in `defaultPolicy`.
- `config.ServerID` is a constant; it should come from stored server configuration.
- Nothing enforces the embedding depth that makes service dispatch work; a generated per-tag interface plus `RegisterXService` would turn the three silent failure modes into compile errors.
- The remaining stub handlers in `internal/server/handlers.go` (DisplayPreferences, QuickConnect, SyncPlay, bitrate test) have no domain yet.
