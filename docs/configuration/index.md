# Configuration

Most deployments should start with the Helm chart's top-level settings and Secrets. Keep credentials out of values files and source control.

## Required

| Setting | Purpose |
|---|---|
| `DATABASE_URL` | PostgreSQL connection string. Required; there is no built-in default. |
| `PUBLISHED_SERVER_URL` | Public hostname clients and downloaders use to reach the server. Set the chart's `hostname` value. |

The chart reads the database URL from a Kubernetes Secret. By default, it looks for a Secret named after the Helm release, with a key named `DATABASE_URL`.

## Optional integrations

- [Metadata and artwork](metadata.md) — TMDB.
- [Libraries](../using-gojellyfin/libraries.md) — Sonarr, Radarr and Lidarr.
- [Single sign-on](single-sign-on.md) — OpenID Connect.
- [Networking](networking.md) — trusted proxies, public URL and routing.

## Configuration reference

The chart README documents every Helm value and the environment variable it controls. Use it as the reference for resource sizing, autoscaling, transcoder slots, extra volumes, tracing and other advanced settings.

[Open the complete Helm values reference](https://github.com/FreekingDean/gojellyfin/blob/main/charts/gojellyfin/README.md).
