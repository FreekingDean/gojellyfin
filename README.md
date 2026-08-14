# gojellyfin

A Go reimplementation of a Jellyfin media server, serving the Jellyfin 10.10.0
HTTP API so that stock `jellyfin-web` and Jellyfin clients can talk to it.

Everything ships as one binary, `cmd/gojellyfin`, with a subcommand each for
`server`, `transcoder`, `worker`, `migrate`, `adduser`, `resetpassword` and
`localizationdata`.

## What works

| | |
|---|---|
| **Movies** | Scanned, played, and their metadata answered |
| **TV** | Series, seasons and episodes, including next-up and resume |
| **Library scanning** | Walks the tree, probes with ffprobe, sweeps what is gone |
| **Background jobs** | Temporal workflows in a separate worker deployment |
| **High availability** | The API is stateless — run as many replicas as you like |
| **Automatic encoding** | Audio a client cannot decode is re-encoded on the fly; the video is copied, never transcoded |
| Live TV | Not implemented |
| Music | Not implemented — the scanner does not walk audio files |
| SyncPlay | Not implemented |

Two of those want a caveat rather than a tick.

**Automatic encoding** covers the common case and no more. A rip whose video a
browser *can* decode but whose audio it cannot — H.264 beside AC-3 or DTS, which
is most of them — is remuxed to fragmented mp4 with the audio re-encoded and the
video copied. Video the client genuinely cannot decode (HEVC, 10-bit H.264,
VC-1) is refused with a 415 rather than transcoded, which is a deliberate
position and not a gap to be filled: keep a second copy on disk instead. The
choice is also not yet driven by the client's declared `DeviceProfile`, so it
reads from a small table of what browsers decode.

**High availability** means the API holds no per-request state and needs no
session affinity — a transcode is one HTTP request from start to finish, so a
second replica breaks nothing. It does not mean the database is replicated.


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
