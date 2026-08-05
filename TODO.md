# TODO

Refactors and cleanups deferred out of a change. One line each.

- Extract the remaining domains into service packages following `internal/server/users`: items, library, configuration, playback, playstate, userdata, branding, localization, system.
- `serverId` and `rootFolderId` are duplicated in `internal/server` and `internal/server/users`; give them one home once more services need them.
- `AuthenticationProviderId` is hardcoded to `"authentik"` in `defaultPolicy`.
- Nothing enforces the embedding depth that makes service dispatch work; a generated per-tag interface plus `RegisterXService` would turn the three silent failure modes into compile errors.
