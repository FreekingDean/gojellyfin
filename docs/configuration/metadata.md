# Metadata and artwork

gojellyfin uses TMDB to identify movies, series and episodes and to look up metadata and artwork.

Set `TMDB_API_KEY` using a TMDB v3 API key or v4 read access token. With the Helm chart, create a Secret containing that key and point `tmdb.existingSecret` at it. The key name defaults to `TMDB_API_KEY`; change `tmdb.secretKey` if your Secret uses another key.

If no TMDB key is configured, the server can still use the titles reported by your media sources, but metadata lookups and artwork are unavailable.

Keep the key in a Kubernetes Secret rather than a Helm values file.
