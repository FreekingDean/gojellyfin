# gojellyfin

A Go reimplementation of a Jellyfin media server, serving the Jellyfin 10.10.0
HTTP API so that stock `jellyfin-web` and Jellyfin clients can talk to it.

Everything ships as one binary, `cmd/gojellyfin`, with a subcommand each for
`server`, `worker`, `migrate`, `adduser`, `resetpassword` and
`localizationdata`.

## What works

| | |
|---|---|
| **Movies** | Scanned, played, and their metadata answered |
| **TV** | Series, seasons and episodes, including next-up and resume |
| **Library scanning** | Walks the tree, probes with ffprobe, sweeps what is gone |
| **Metadata** | Movies, series and episodes identified against TMDB, with your own `TMDB_API_KEY` |
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

Needs a reachable Postgres. `DATABASE_URL` is required — the binary carries no
default — and the `Makefile` supplies the development one, so `make` targets
work out of the box and an explicit `DATABASE_URL` still wins.

```sh
make dev     # watch and restart on :8081
make build test lint
echo hunter2 | DATABASE_URL=... go run ./cmd/gojellyfin adduser Dean   # bootstrap the first user
```

`CLAUDE.md` carries the architecture and the reasoning behind it.

## Deployment

`charts/gojellyfin` is a Helm chart for the shape this runs in. Create the
Secret holding `DATABASE_URL` — the chart carries no credential — and install:

```sh
kubectl create secret generic gojellyfin --from-literal DATABASE_URL='postgres://…'
helm install gojellyfin ./charts/gojellyfin
```

It is one image run as up to four workloads: the API, the transcode pods, the
Temporal worker and nginx serving `jellyfin-web`. The transcode pods are the
same `server` subcommand as the API — what makes them the transcoders is the
route, which sends `/Videos` and `/Audio` to them, so the pod that serves the
stream is the pod that runs ffmpeg.

The one thing they must agree on is the media. The API hands ffmpeg the item's
filesystem path straight out of the database, so the volume has to be mounted at
the same path in every pod that reads it — read-write for the API, which deletes
files, read-only everywhere else.

Migrations never run at startup unless `MIGRATE_ON_START=true`, which is unsafe
with rolling replicas, so the chart does not offer it. Run `gojellyfin migrate`
as a one-off or set `migration.enabled=true` for a pre-upgrade hook Job;
`charts/gojellyfin/README.md` documents the tradeoff and every other value.
