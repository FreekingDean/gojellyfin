# Troubleshooting

Start with the symptom, then check the component responsible. The API, streaming pods, worker and web client are separate workloads in a Helm deployment.

## The server does not start

- Confirm `DATABASE_URL` is present and points to a reachable PostgreSQL database.
- Check the API pod logs for connection or migration errors.
- Remember that migrations do not run automatically at startup. Run `gojellyfin migrate` as a one-off or enable the chart migration hook for an upgrade that needs one.

## I cannot sign in

- On a new database, create the first account with `gojellyfin adduser`.
- Check that the browser is reaching the hostname and scheme the server advertises.
- For OpenID Connect, verify the issuer URL, client ID, redirect URI and any group restrictions.
- Use `gojellyfin resetpassword` if a local account's password needs resetting.

## My library is empty or out of date

- Confirm the Radarr, Sonarr or Lidarr source is reachable and its API key is valid.
- Confirm the worker is enabled and connected to Temporal if background scans are expected.
- Check the source manager can reach the configured public server URL to call webhooks.
- A scheduled refresh is available if a webhook was missed.

## Playback fails

- Check whether the selected client supports the file's codecs and container.
- Check whether playback would require picture encoding above `transcoderMaxSourceHeight`.
- Confirm the streaming route reaches the streaming Service and that the pods have free transcoder slots.
- Inspect ffmpeg errors in the server logs.

## The web dashboard or plugin pages are missing

If you use a custom gateway, do not route all of `/web` to the static web workload. Some Jellyfin API endpoints also use that prefix. Route `/web/ConfigurationPage` and `/web/ConfigurationPages` to the API, including the lower-case request spellings.

## A migration appears to have succeeded but clients ask me to sign in

Check that `SERVER_ID` is set to the id printed by `scripts/jellyfinmigrate`. Clients associate their stored token with the server id.
