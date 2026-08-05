# TODO

Refactors and cleanups deferred out of a change. One line each.

- `store.Store` grows a method per operation; split into per-domain interfaces that `Store` composes once it passes ~5 domains.
- `internal/server` imports `internal/http/middleware` for the request context accessors — move them somewhere transport-neutral if the layering starts to hurt.
- `internal/server/system.go` declares a `VERSION` var that shadows `system.VERSION`; fold into stored server config in the config phase.
- `AuthenticationProviderId` is hardcoded to `"authentik"` in `defaultPolicy`.
