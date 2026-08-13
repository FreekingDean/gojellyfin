# gojellyfin

A Go reimplementation of a Jellyfin media server, serving the Jellyfin 10.10.0
HTTP API so that stock `jellyfin-web` and Jellyfin clients can talk to it.

Everything ships as one binary, `cmd/gojellyfin`, with a subcommand each for
`server`, `transcoder`, `migrate`, `adduser`, `resetpassword` and
`localizationdata`.

## Development

Needs a reachable Postgres; `DATABASE_URL` overrides the default DSN.

```sh
make dev     # watch and restart on :8081
make build test lint
echo hunter2 | go run ./cmd/gojellyfin adduser Dean   # bootstrap the first user
```

`CLAUDE.md` carries the architecture and the reasoning behind it.

## Deployment

`deploy/` holds plain Kubernetes manifests for the shape this runs in — no Helm
and no Kustomize, so `kubectl apply -f deploy/` is the whole of it once
`secret.yaml.example` has been copied, filled in and applied.

It is two processes out of the one image. `gojellyfin server` answers the API on
:8081 and owns the database. `gojellyfin transcoder` runs ffmpeg on behalf of
the API and streams the output back over :8082, as its own workload so that a
runaway encode cannot starve the request path; it holds no database credentials
at all. The API finds the workers through `TRANSCODER_WORKERS` and round robins
over them, authenticating with a shared `TRANSCODER_TOKEN`.

The one thing the two must agree on is the media. The API sends a worker the
item's filesystem path straight out of the database, so the media volume has to
be mounted at the same path in both — read-write for the API, read-only for the
workers.

Migrations never run at startup unless `MIGRATE_ON_START=true`, which is unsafe
with rolling replicas. `deploy/migrate-job.yaml` runs `gojellyfin migrate` as a
one-off Job instead; the migrations are embedded in the binary, so the Job and
the API must run the same image tag.
