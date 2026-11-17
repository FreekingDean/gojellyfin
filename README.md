# gojellyfin

A Go reimplementation of Jellyfin's server API. It serves the Jellyfin 12.0.0 HTTP API, so the standard `jellyfin-web` interface and Jellyfin clients can connect to it.

## What you can do

| Feature | What it means |
|---|---|
| **Movies, TV and music** | Build your library from Radarr, Sonarr and Lidarr. Includes next-up, resume and lyrics. |
| **Metadata and artwork** | Match movies, series and episodes with TMDB. Add your own `TMDB_API_KEY` (a v3 API key or v4 read access token). |
| **Playback** | Play compatible files directly. When needed, gojellyfin remuxes, converts audio or transcodes video to 1080p H.264 over HLS. |
| **Live TV** | Create channels from your own titles and play them as live HLS. |
| **SyncPlay** | Watch together across server replicas. |
| **Requests and reports** | Request missing titles through Radarr or Sonarr, or report a broken copy so another can be requested. |
| **Sign-in** | Passwords, TV-based Quick Connect, OpenID Connect single sign-on and API keys. |
| **Background work** | Run Temporal workflows in a separate worker deployment. |
| **Multiple replicas** | Run interchangeable server pods. Postgres stores shared session and coordination state. |
| **Admin app** | Open the admin interface at `/admin`, alongside the standard Jellyfin dashboard. |

### Playback and compatibility

The server chooses the best available file for each client's `DeviceProfile`. It prefers a file the client can play as-is, then uses remuxing or audio conversion, and only transcodes the picture as a last resort.

Picture transcoding is limited to sources at or below `TRANSCODER_MAX_SOURCE_HEIGHT` (1080 by default). If a title only has 4K copies and needs a picture transcode, playback is refused rather than starting an unsupported encode.

Live TV does not use a tuner and does not support recordings. Remote control of another session is not available.

### Running more than one server

Server pods can handle requests interchangeably. Sessions, Quick Connect, SyncPlay and websocket delivery are stored in Postgres, and HLS segments are generated independently. This does not replicate Postgres itself; database availability remains your responsibility.

## Get started

gojellyfin needs a reachable PostgreSQL database. Set `DATABASE_URL` before starting the server; the binary does not include a default database connection string.

For local development, the Makefile provides a development database configuration:

```sh
make dev       # Run migrations, build, and serve on :8081
make build
make test
make lint
```

To create the first user, run:

```sh
echo 'your-password' | DATABASE_URL='postgres://…' go run ./cmd/gojellyfin adduser your-username
```

The binary also includes the `server`, `worker`, `migrate`, `adduser`, `resetpassword` and `localizationdata` commands.

## Deploy with Helm

The project includes a Helm chart at [`charts/gojellyfin`](charts/gojellyfin). Before installing, create a Kubernetes Secret containing your database URL:

```sh
kubectl create secret generic gojellyfin \
  --from-literal DATABASE_URL='postgres://user:password@host:5432/gojellyfin?sslmode=require'

helm install gojellyfin oci://ghcr.io/freekingdean/charts/gojellyfin --version 0.0.1
```

The chart does not store credentials in its values. See the [Helm chart guide](charts/gojellyfin/README.md) for the full install, storage, routing, migration and configuration instructions.

### How the deployment is organised

The chart runs one image in up to four workloads:

- **API** handles normal Jellyfin requests.
- **Streaming** handles `/Videos` and `/Audio`. These pods run the same server command as the API, but the gateway routes stream requests to them so ffmpeg runs on the pod serving the stream.
- **Worker** runs Temporal background workflows.
- **Web** serves `jellyfin-web` through nginx.

Your media stays on storage you provide. The chart does not create a media volume. Every pod that reads media must see the same files at the same paths. The API needs write access because deleting an item is an API operation; other workloads mount the media read-only.

New downloads can appear in the library as soon as they are imported. When you save a Sonarr, Radarr or Lidarr connection on the admin app's Libraries screen, gojellyfin registers a webhook with that service. The webhook refreshes the affected item after an import, upgrade, rename or delete. This requires `PUBLISHED_SERVER_URL` (the chart's `hostname`) to be reachable by the downloader. If automatic registration fails, the Libraries screen explains why and shows the details needed to add the webhook manually. Scheduled library refreshes still provide a fallback.

Database migrations do not run automatically at server startup. Run `gojellyfin migrate` as a one-off, or configure `migration.enabled=true` in the chart to run a pre-upgrade Job. The chart guide explains the trade-offs.

## Move from Jellyfin

Use [`scripts/jellyfinmigrate`](scripts/jellyfinmigrate) to bring an existing Jellyfin installation into gojellyfin. It runs once from a source checkout against the old Jellyfin data directory and your gojellyfin database. It reads the old data and does not modify it.

The script supports Jellyfin 10.10 and 10.11. It migrates:

- Users, including their IDs, password hashes, policy and playback preferences.
- Signed-in devices and access tokens, so existing clients can stay signed in.
- API keys. Jellyfin does not record who created a key, so migrated keys belong to the oldest administrator.
- Watch state: played status, play count, resume position, favourites and ratings.

Libraries and metadata are not migrated; gojellyfin rebuilds those from Sonarr and Radarr. Restricted-library assignments, parental ratings and display preferences are not carried over.

### Migration steps

1. **Set up the new library first.** Connect your downloaders and let a library refresh finish so titles exist for the watch state to attach to.
2. **Do a dry run.** Copy Jellyfin's data directory (the one containing `jellyfin.db`, `library.db` and `device.txt`; in the official image this is `/config/data`) and run:

   ```sh
   DATABASE_URL='postgres://…' go run ./scripts/jellyfinmigrate --from ./jellyfin-data --dry-run
   ```

   Review the output, especially titles the script could not match.
3. **Stop Jellyfin** and make a fresh copy of its data directory so it cannot change during migration.
4. **Run the migration** using the same command without `--dry-run`. The import runs in one transaction; if it fails, it writes nothing.
5. **Reuse the old server ID.** The script prints Jellyfin's server ID. Set it as `SERVER_ID` (or the chart's `serverId` value) and restart gojellyfin. Clients associate their saved token with this ID.
6. **Point your existing hostname at gojellyfin.** Clients can then find the same server ID and reuse their existing sign-in.

You can safely run the migration again. Existing users, tokens and API keys are left alone, and watch state is merged rather than overwritten. A title that could not be matched on the first run may be picked up after a later library scan.

A few cases need attention:

- If a gojellyfin account already has the same username as a Jellyfin user, that user and their devices are skipped. Rename or remove the existing account before retrying; matching names do not prove they are the same person.
- A Jellyfin user without a password (for example, one authenticated by a plugin) will need a password set with `gojellyfin resetpassword`.
- Users restricted to selected Jellyfin libraries will need their library access configured again in the dashboard.
- Migrated access tokens remain valid. If an old token was exposed, it remains usable after migration; changing that user's password revokes their sessions.

## Screenshots

The [docs site](https://freekingdean.github.io/gojellyfin/) covers installing, configuring and using gojellyfin. It is built from the Markdown in `docs/`.

The [screenshot gallery](https://freekingdean.github.io/gojellyfin/main/) shows gojellyfin in `jellyfin-web` and the standard Android TV app. The browser suite renders on every merge; emulator screenshots are updated nightly.

## Contributing

See the development commands above to build, test and lint the project. The architecture and implementation notes are maintained separately from this user guide.
