# Connect your libraries

gojellyfin builds its movie, TV and music libraries from your existing media managers rather than asking you to import a separate metadata database.

The supported sources described by the project are Radarr, Sonarr and Lidarr.

## Add a source

In the admin app, open **Libraries** and add the relevant source. Provide the source address and API key. The server tests the connection before saving it.

- Radarr provides movie libraries.
- Sonarr provides series and episode libraries.
- Lidarr provides music libraries.

The API key is sensitive. Store it as a Secret in Kubernetes; the chart's `sources.existingSecret` points at a Secret whose keys match the source API key environment variable names configured for the sources.

## Keep changes in sync

When you save a source, gojellyfin attempts to register a webhook named `gojellyfin` in the source manager. Imports, upgrades, renames and deletes can then refresh the affected item without waiting for a full scan.

Webhook registration requires `PUBLISHED_SERVER_URL` to be set and reachable from the source manager. If automatic registration fails, the Libraries screen reports the reason. You can also create the webhook manually in the source manager's **Settings → Connect** area.

A scheduled refresh remains in place as a fallback.

## Background scanning

Library scans and other background work run in the Temporal worker. In the Helm chart, the worker is disabled by default. To use background workflows, configure a Temporal endpoint and enable the worker. See the chart README for `temporal` and `worker` settings.

## Metadata

TMDB is used to identify movies, series and episodes and to retrieve metadata and artwork. See [Metadata and artwork](../configuration/metadata.md).
