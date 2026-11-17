# gojellyfin

**Your media. The Jellyfin apps you already use. A different server underneath.**

gojellyfin is a Go reimplementation of the Jellyfin media server. It speaks the
Jellyfin HTTP API, so familiar clients — including the web app and Android TV —
can connect to it.

These docs are for people installing, running and using gojellyfin. They focus
on what works today, what you need to provide, and how to get unstuck.

## Start here

<div class="grid cards" markdown>

-   :material-rocket-launch: **Getting started**

    Set up a fresh instance, create the first account and sign in.

    [Get started](getting-started/index.md)

-   :material-kubernetes: **Deployment**

    Install and upgrade gojellyfin on Kubernetes with Helm.

    [Deploy gojellyfin](deployment/helm.md)

-   :material-tune: **Configuration**

    Configure PostgreSQL, TMDB, OpenID Connect and networking.

    [Configure your server](configuration/index.md)

-   :material-play-circle: **Using gojellyfin**

    Connect media sources and understand playback, Live TV and SyncPlay.

    [Explore features](using-gojellyfin/index.md)

-   :material-swap-horizontal: **Migration**

    Bring users, devices and watch state over from Jellyfin.

    [Plan a migration](migration/from-jellyfin.md)

-   :material-life-buoy: **Troubleshooting**

    Work through common startup, sign-in, library and playback problems.

    [Get help](troubleshooting/index.md)

</div>

## What to expect

- Uses the Jellyfin API and familiar Jellyfin clients.
- Builds movie, TV and music libraries from Radarr, Sonarr and Lidarr.
- Uses TMDB for metadata and artwork when configured.
- Supports direct play, remuxing, audio conversion and bounded video encoding.
- Includes Live TV channels built from library titles and SyncPlay groups.
- Stores shared session and playback coordination state in PostgreSQL, allowing multiple server replicas.

A few important boundaries: gojellyfin does not provide TV tuners or recordings, and it does not replicate your PostgreSQL database. Video encoding is limited by a configurable source-height ceiling; a 4K file that needs picture encoding may be refused rather than transcoded.

## Project links

- [Source code on GitHub](https://github.com/FreekingDean/gojellyfin)
- [Screenshot gallery](https://freekingdean.github.io/gojellyfin/main/)
- [Writing and improving these docs](contributing/writing-docs.md)
