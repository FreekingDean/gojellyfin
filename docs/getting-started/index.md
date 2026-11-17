# Getting started

gojellyfin is a media server that speaks the Jellyfin API. That means you can use familiar Jellyfin clients, including the web app and Android TV app, while gojellyfin handles the server side.

This guide gets a fresh instance running. If you're already comfortable with Kubernetes, jump straight to [Deploy with Helm](../deployment/helm.md).

## Before you start

You'll need:

- A PostgreSQL database that the server can reach.
- A place where your media files are stored.
- A TMDB API key if you want artwork and metadata lookups.
- Sonarr, Radarr or Lidarr if you want gojellyfin to build libraries from your existing collection.
- A hostname and HTTPS route if other devices will connect over your network or the internet.

The Helm chart does not create a database, store credentials, or provision media storage. Those are services you provide.

## The quickest route

1. Prepare PostgreSQL and create a Secret containing the connection string.
2. Install gojellyfin using the [Helm guide](../deployment/helm.md).
3. Mount your media at the same filesystem path in every server pod.
4. Apply database migrations.
5. Create the first administrator account.
6. Sign in through the web client and add your media sources.

A new database has no users, so the first account must be created from the command line. See [First login](first-login.md).

## How the pieces fit together

The API handles sign-in, library browsing and the Jellyfin-compatible endpoints. Streaming requests can be sent to a separate pool of server pods so video work does not compete with ordinary API traffic. A web workload serves the Jellyfin web client. Background library work runs in a separate Temporal worker when enabled.

All server replicas share PostgreSQL. Media files are mounted into the pods; gojellyfin does not copy your collection into the database.

## Pick your next step

- [Deploy with Helm](../deployment/helm.md) — install on Kubernetes.
- [First login](first-login.md) — create the initial administrator and sign in.
- [Connect your libraries](../using-gojellyfin/libraries.md) — add Sonarr, Radarr and Lidarr.
- [Configuration](../configuration/index.md) — database, metadata, SSO and networking settings.
