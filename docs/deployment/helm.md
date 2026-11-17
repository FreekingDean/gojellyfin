# Deploy with Helm

The supported deployment shape is the Helm chart in `charts/gojellyfin`. It deploys one gojellyfin image as separate workloads for the API, streaming, and (optionally) background jobs, plus a web client.

The chart is published as an OCI artifact at `ghcr.io/freekingdean/charts/gojellyfin`. Each release publishes the chart and image with the same version.

## What you need first

- A Kubernetes cluster with Helm 3.
- A reachable PostgreSQL database.
- A Kubernetes Secret containing the database URL.
- A media volume you can mount into the pods.
- A Gateway API implementation if you want the chart to create HTTPRoutes.

The chart does not create PostgreSQL, a PersistentVolumeClaim, or credentials.

## Install

Create the database Secret. The default Secret name is the Helm release name:

```sh
kubectl create secret generic gojellyfin \
  --from-literal DATABASE_URL='postgres://user:password@db.example.com:5432/gojellyfin?sslmode=require'
```

Install the chart, setting your public hostname and existing media claim:

```sh
helm install gojellyfin oci://ghcr.io/freekingdean/charts/gojellyfin \
  --version 0.0.1 \
  --set hostname=media.example.com \
  --set-json 'media.volume={"persistentVolumeClaim":{"claimName":"media"}}'
```

Use a release version that exists in the package registry. The example version is illustrative; check the published chart versions before installing.

If your Secret has a different name, set `database.existingSecret`. The chart reads the `DATABASE_URL` key from that Secret; it does not put the URL in Helm values.

## Configure the media mount

Set `media.volume` to the Kubernetes volume source for your existing media. The default is an emptyDir, which is useful for a smoke test but will not contain your library.

Every API and streaming pod must see the same files at the same paths. The API needs write access because it can delete files; the other workloads mount media read-only.

For more than one media volume, use `media.extraVolumes`. Set `media.directories` if you want to control which mounted paths appear when adding a library.

## Routing

Set `httpRoute.enabled=true` and provide `httpRoute.parentRefs` to create Gateway API routes. The chart routes:

- `/` to the API.
- `/Videos` and `/Audio` to the streaming workload.
- `/web` to the web client, with specific API endpoints under `/web` routed back to the API.

If you use another proxy, configure equivalent routing yourself. In particular, do not send every `/web` request to the static web client: some Jellyfin API endpoints also live under that path.

## Apply the schema and create an administrator

Migrations are not run automatically when a pod starts. For a release that needs a migration, either enable the chart's migration hook:

```yaml
migration:
  enabled: true
```

Or run `gojellyfin migrate` as a one-off job using the same image version as the release. The chart's migration hook is a pre-install/pre-upgrade Job.

Then create the first user:

```sh
kubectl exec -i deploy/gojellyfin -- gojellyfin adduser Dean
```

See [First login](../getting-started/first-login.md).

## Workloads at a glance

| Workload | Enabled by default | Purpose |
|---|---:|---|
| API | Yes | Jellyfin-compatible API |
| Streaming | Yes | Handles video and audio stream requests and runs ffmpeg |
| Worker | No | Temporal background workflows, including library scans |
| Web | Yes | Serves the Jellyfin web client |

Streaming can be disabled; in that case the API workload handles those requests too. The worker needs a Temporal endpoint and is not enabled by default.

## Upgrading and rollback

Pin the chart version when installing or upgrading. The published chart's app version points at the matching container image, keeping the API, streaming pods, worker and migration Job on the same build.

```sh
helm upgrade gojellyfin oci://ghcr.io/freekingdean/charts/gojellyfin \
  --version <version>
```

For the full list of values and advanced options, see the [chart README](https://github.com/FreekingDean/gojellyfin/blob/main/charts/gojellyfin/README.md).
