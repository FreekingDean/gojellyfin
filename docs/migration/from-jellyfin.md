# Move from Jellyfin

The `scripts/jellyfinmigrate` tool moves selected account and watch-state data from an existing Jellyfin installation into gojellyfin. It is run once from a gojellyfin checkout and connects to both the source Jellyfin data directory and the target PostgreSQL database.

It reads the Jellyfin data directory; it does not modify it. Jellyfin 10.10 and 10.11 are supported.

## What gets carried over

- User accounts, ids, password hashes, policy and playback preferences.
- Signed-in devices and access tokens.
- API keys.
- Watch state: played status, play count, resume position, favourites and ratings.

Titles are matched by TMDB id first, then IMDb or TVDB id, then file path.

## What does not get carried over

Libraries and metadata are rebuilt from your media sources. Restricted-library assignments, parental ratings and display preferences are not migrated.

## Before you start

1. Install gojellyfin and connect your Radarr, Sonarr and/or Lidarr sources.
2. Let the library refresh finish so titles exist in the target database.
3. Make a copy of Jellyfin's data directory. In the official container this is under `/config/data`; it contains files such as `jellyfin.db`, `library.db` and `device.txt`.
4. Rehearse the migration with `--dry-run`.

## Dry run

From a gojellyfin checkout:

```sh
DATABASE_URL='postgres://user:password@db.example.com:5432/gojellyfin?sslmode=require' \
  go run ./scripts/jellyfinmigrate --from ./jellyfin-data --dry-run
```

The dry run reports what it would import and titles it could not match. It does not write to either database.

## Cutover

1. Stop Jellyfin so the source data is no longer changing.
2. Make a fresh copy of the Jellyfin data directory.
3. Run the same command without `--dry-run`.
4. Set gojellyfin's `SERVER_ID` to the server id printed by the migration tool. With Helm, set `serverId` to this value.
5. Point your existing hostname or proxy at gojellyfin.

Keeping the same server id matters: Jellyfin clients associate their saved access token with that id. If the id changes, clients may ask users to sign in again even though the tokens were imported.

## Safe to rerun

The migration is designed to be rerun. Existing users, tokens and API keys are left alone, and watch state is merged rather than blindly overwritten. A title that did not match during the first run may match after the library has been refreshed.

A carried access token remains valid on the new server. Treat the old Jellyfin data directory as sensitive, and revoke sessions by changing a user's password if you suspect a token has leaked.
