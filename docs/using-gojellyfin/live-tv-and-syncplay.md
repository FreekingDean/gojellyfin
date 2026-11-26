# Live TV and SyncPlay

## Live TV

gojellyfin can expose channels built from titles already in your library, with a guide and live HLS playback.

This is not a tuner integration. There are no tuner devices, live broadcast inputs or recording features described as part of the current implementation.

## SyncPlay

SyncPlay lets people in a group watch together. Group state is stored in PostgreSQL, so a group is not tied to one API process and can continue to work when requests land on different replicas.

## Current boundaries

The project does not offer remote control of another user's playback session. Live TV does not include tuner support or recordings.
