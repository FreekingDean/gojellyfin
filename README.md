# gojellyfin

A Go reimplementation of a Jellyfin media server, serving the Jellyfin 12.0.0
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
| **Metadata** | Movies, series and episodes identified against TMDB, with your own `TMDB_API_KEY` (a v3 API key or a v4 read access token) |
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

The [screenshot gallery](https://freekingdean.github.io/gojellyfin/main/) shows
what `main` looks like in `jellyfin-web` and in the stock Android TV app, as the
end-to-end suites last rendered it: the browser on every merge, the emulators
nightly.

## Deployment

`charts/gojellyfin` is a Helm chart for the shape this runs in. Create the
Secret holding `DATABASE_URL` — the chart carries no credential — and install:

```sh
kubectl create secret generic gojellyfin --from-literal DATABASE_URL='postgres://…'
helm install gojellyfin oci://ghcr.io/freekingdean/charts/gojellyfin --version 0.0.1
```

Every merge to `main` publishes the image and the chart at the same version.

It is one image run as up to four workloads: the API, the streaming pods, the
Temporal worker and nginx serving `jellyfin-web`. The streaming pods are the
same `server` subcommand as the API — what makes them the streaming pods is the
route, which sends `/Videos` and `/Audio` to them, so the pod that serves the
stream is the pod that runs ffmpeg.

The one thing they must agree on is the media, which is yours: the chart mounts
whatever volume `media.volume` names and creates no storage of its own. The API
hands ffmpeg the item's filesystem path straight out of the database, so it has
to be the same media at the same path in every pod — read-write for the API,
which deletes files, read-only everywhere else.

### New downloads show up at once

Saving a Sonarr, Radarr or Lidarr on the admin app's Libraries screen also adds a
webhook named `gojellyfin` under its Settings, Connect, so an import, an
upgrade, a rename or a delete refreshes that one movie, series or artist straight away
instead of waiting for the next library refresh. It needs `PUBLISHED_SERVER_URL`
(the chart's `hostname`), and the downloader has to be able to reach that
address: the instance tests the webhook before it saves it, and the card on the
Libraries screen says why when it refused.

If it cannot be registered, add it by hand: **Settings → Connect → + → Webhook**,
with On Import (On Release Import in Lidarr), On Upgrade, On Rename and every delete ticked, method `POST`,
URL `<your server>/webhooks/<source id>`, username `gojellyfin` and the password
the Libraries card shows for that downloader. The scheduled refresh still runs,
so a missed call is caught up on the next one.

Migrations never run at startup unless `MIGRATE_ON_START=true`, which is unsafe
with rolling replicas, so the chart does not offer it. Run `gojellyfin migrate`
as a one-off or set `migration.enabled=true` for a pre-upgrade hook Job;
`charts/gojellyfin/README.md` documents the tradeoff and every other value.

## Moving over from Jellyfin

`scripts/jellyfinmigrate` carries an existing Jellyfin install into gojellyfin
so nobody has to sign in again. It is a script rather than a part of the server:
it runs once, from a checkout, against the two databases.

It reads Jellyfin's data directory — the one holding `jellyfin.db`, `library.db`
and `device.txt`, which is `/config/data` in the official image — and never
writes to it. Jellyfin 10.10 and 10.11 are both read. It carries:

- **Users**, with the ids Jellyfin gave them, their password hashes, their
  policy and their playback preferences. A Jellyfin password keeps working.
- **Signed-in devices and their access tokens**, so a phone, a television and a
  browser that were signed in stay signed in.
- **API keys.**
- **Watch state**: played, play count, resume position, favourites and ratings.
  A title is matched by its TMDB id, then by its IMDb or TVDB id, then by its
  file path.

Not carried: libraries and metadata, which gojellyfin fills from Sonarr and
Radarr; which libraries a restricted user may see, because the library ids
differ; parental ratings; and display preferences.

### Cutover

1. **Fill gojellyfin first.** Create the libraries, bind the sources and let a
   refresh finish, so the titles exist for the watch state to land on.
2. **Rehearse.** Copy Jellyfin's data directory somewhere the script can read
   and run it with `--dry-run`. It prints what it would carry and every title it
   could not match, and writes nothing.

   ```sh
   DATABASE_URL='postgres://…' go run ./scripts/jellyfinmigrate --from ./jellyfin-data --dry-run
   ```

3. **Stop Jellyfin**, so nothing changes underneath the copy, and copy the data
   directory again.
4. **Run it for real**: the same command without `--dry-run`. It is one
   transaction, so a failure writes nothing.
5. **Give gojellyfin Jellyfin's server id.** The script ends by printing it. Set
   `serverId` in the chart (`SERVER_ID` in the environment) and restart. A
   client keeps its token filed under the server's id, so without this every
   client asks for a sign-in however many tokens were carried.
6. **Point the hostname at gojellyfin.** Clients come back to the same address,
   find the same server id and the token they already hold, and carry on.

Running it again is safe. A user, a token and an api key that are already here
are left alone, and watch state is merged rather than overwritten: played and
favourite if either side says so, and the further of the two play counts and
positions. So a title that had no match the first time is picked up by a later
run once a scan has it.

Three things it reports rather than guesses at. A Jellyfin user whose name an
account here already holds is skipped with their devices, since a matching name
is not a matching person; rename or delete the account here and run it again. A
user with no Jellyfin password, such as one signed in through a plugin, has none
here until `gojellyfin resetpassword`. And a user Jellyfin limited to chosen
libraries sees none here until theirs are ticked in the dashboard.

A carried token is as good here as it was there, so a token that leaked from the
old install signs in to the new one too. Changing a password revokes that
user's sessions.
