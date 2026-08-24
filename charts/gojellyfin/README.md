# gojellyfin

A Helm chart for [gojellyfin](https://github.com/FreekingDean/gojellyfin), a Go
reimplementation of a Jellyfin media server.

## Install

The chart never carries a credential. `DATABASE_URL` is required — the binary
has no default and fails at start without one — so create the Secret first:

```sh
kubectl create secret generic gojellyfin \
  --from-literal DATABASE_URL='postgres://user:password@host:5432/gojellyfin?sslmode=require'

helm install gojellyfin ./charts/gojellyfin \
  --set hostname=media.example.com \
  --set-json 'media.volume={"persistentVolumeClaim":{"claimName":"media"}}'
```

The release name decides the Secret's default name: an install named `gojellyfin`
looks for a Secret named `gojellyfin`, and `database.existingSecret` names a
different one. Nothing in `values.yaml` ever holds the URL.

Then apply the schema (see [Migrations](#migrations)) and create the first user
— a fresh database has no way in through the API:

```sh
kubectl exec -i deploy/gojellyfin -- gojellyfin adduser Dean
```

## What it deploys

One image, `ghcr.io/freekingdean/gojellyfin`, as up to four workloads:

| Workload | Command | Enabled by default | What it is |
|---|---|---|---|
| `api` | `server` | yes | The Jellyfin API on :8081 |
| `streaming` | `server` | yes | The same command, routed the streaming paths |
| `worker` | `worker` | no | Temporal workflows — the library scan |
| `web` | — | yes | nginx serving `jellyfin-web`, copied out of the all-in-one image by an init container |

The streaming pods are not a second binary or a second protocol. They are the
same `server` command, and what makes them the streaming pods is the HTTPRoute:
`/Videos` and `/Audio` go to them, everything else to the API. The pod that
serves the stream is the pod that runs ffmpeg, so no encode crosses the network
twice. Disable `streaming` and the API serves those paths itself.

The media is yours, not the chart's — it creates no PersistentVolumeClaim and
owns no storage. Point `media.volume` at whatever the media already lives on.

## Values

### Image and naming

| Value | Default | Description |
|---|---|---|
| `nameOverride` | `""` | Replaces the chart name in resource names and labels |
| `fullnameOverride` | `""` | Replaces the whole generated name |
| `image.repository` | `ghcr.io/freekingdean/gojellyfin` | The one image every gojellyfin workload runs |
| `image.tag` | `""` | Empty takes the chart's `appVersion`. The API, the worker and the migration Job all use it — the migrations are embedded in the binary, so two tags mean the schema one image applied and the schema the other expects can differ |
| `image.pullPolicy` | `IfNotPresent` | |
| `imagePullSecrets` | `[]` | Applied to every pod |

### Server environment

Every variable below is read by `internal/env`, which is the only package that
reads the environment.

| Value | Environment variable | Default | Description |
|---|---|---|---|
| `hostname` | `PUBLISHED_SERVER_URL` | `""` | The name clients reach the server by. The routes match it, and the public endpoints advertise `https://<hostname>`. Empty advertises no address at all — a client prefers `LocalAddress` when it believes it shares a network with the server, so naming one it cannot reach sends it nowhere |
| `httpPort` | `HTTP_PORT` | `8081` | What the server listens on. The container port, the Service target and the probes follow it |
| `database.existingSecret` | `DATABASE_URL` | `""` | Secret holding the URL. Empty means the release's own fullname |
| `database.secretKey` | — | `DATABASE_URL` | Key within that Secret |
| `tmdb.existingSecret` | `TMDB_API_KEY` | `""` | Secret holding the TMDB key. Empty sets no key and metadata lookups fail |
| `tmdb.secretKey` | — | `TMDB_API_KEY` | Key within that Secret |
| `cors.enabled` | `CORS_ORIGINS` | `false` | Off reflects whatever origin asks; a literal `*` is not equivalent, because browsers reject it on credentialed requests |
| `cors.origin` | | `""` | The one origin allowed when enabled |
| `tracing.otlpEndpoint` | `OTEL_EXPORTER_OTLP_ENDPOINT` | `""` | Empty exports no traces |
| `temporal.hostPort` | `TEMPORAL_HOSTPORT` | `""` | Empty turns background work off and the server still starts |
| `temporal.namespace` | `TEMPORAL_NAMESPACE` | `""` | Required once `hostPort` is set; the binary refuses to invent one, and so does the chart |
| `api.transcoderJobs`, `streaming.transcoderJobs` | `TRANSCODER_JOBS` | `""` | Concurrent encodes on one pod. Empty follows the cpu request |
| `api.transcoderStallTimeout`, `streaming.transcoderStallTimeout` | `TRANSCODER_STALL_TIMEOUT` | `30s` | How long a write to a client that vanished without closing may block before the encode is killed |

`MIGRATE_ON_START` is deliberately not exposed — see [Migrations](#migrations).

### TRANSCODER_JOBS and the cpu request

One encode saturates about a core, so `transcoderJobs` left empty is the
workload's cpu request rounded down, and at least 1: 200m gets one slot, 2 gets
two. Past its slots the pod answers 503 with a `Retry-After` and the gateway
retries against another pod, and that refusal is the whole of the load
balancing — there is no pool and no shared state.

Raising the request raises the slots with it. Setting `transcoderJobs`
explicitly overrides that in either direction, including above the request if
that is what you want.

The chart always sets the variable, because leaving it unset makes the binary
fall back to the number of cores on the *node*, which is not what the pod may
use.

### Workloads

Each of `api`, `streaming`, `worker` and `web` takes the same shape:
`replicaCount`, `resources`, `podAnnotations`, `podSecurityContext`,
`securityContext`, `nodeSelector`, `tolerations` and `affinity`.

| Value | Default | Description |
|---|---|---|
| `api.replicaCount` | `2` | The API holds no per-request state and needs no session affinity |
| `api.resources` | 200m/256Mi, limits 1 cpu/1Gi | |
| `api.mediaReadOnly` | `false` | Writable because `DeleteItem` removes the file |
| `api.livenessProbe`, `api.readinessProbe` | `periodSeconds` 20 / 10 | Merged onto a fixed `httpGet` of `/System/Ping` — the only endpoint that is both public and free of any database access, so it reports the process rather than its dependencies |
| `api.service.type`, `.port`, `.annotations` | `ClusterIP`, `80`, `{}` | An Ingress needs a controller this chart cannot guess; `httpRoute` below is the supported edge |
| `api.securityContext` | no privilege escalation, drops ALL | No `runAsNonRoot`: the image's `USER` is a name rather than a uid and kubelet refuses to start a container whose non-rootness it cannot check numerically. The image drops privilege on its own |
| `streaming.enabled` | `true` | Disabled, the API serves `/Videos` and `/Audio` itself |
| `streaming.replicaCount` | `3` | |
| `streaming.resources` | 2 cpu/512Mi, limits 2 cpu/2Gi | The request is what decides the encode slots |
| `streaming.mediaReadOnly` | `true` | Nothing here deletes a file |
| `streaming.livenessProbe` | period 20, timeout 5, 6 failures | Looser than the API's: a pod at capacity is busy, not broken |
| `streaming.readinessProbe`, `streaming.service.*` | as the API's | |
| `worker.enabled` | `false` | Needs `temporal.hostPort`; the chart refuses to render an enabled worker with nothing to dial, because it would only crash loop |
| `worker.replicaCount` | `1` | |
| `worker.resources` | 200m/256Mi, limits 1 cpu/1Gi | |
| `worker.mediaReadOnly` | `true` | The scan reads the tree and probes files with ffprobe |
| `web.enabled` | `true` | |
| `web.replicaCount` | `2` | |
| `web.client.repository`, `.tag` | `jellyfin/jellyfin`, `10.10.0` | `jellyfin-web` is not published on its own — no image, no npm package, no built assets in its releases — so an init container copies it out of the all-in-one image into an emptyDir. Pin the tag to the API version the server implements; a newer client calls endpoints that answer 501 |
| `web.client.resources` | 50m/64Mi, limit 256Mi | |
| `web.image.repository`, `.tag` | `nginxinc/nginx-unprivileged`, `alpine` | The unprivileged image listens on 8080 and owns the directories nginx writes to, which is what lets `runAsNonRoot` hold |
| `web.containerPort` | `8080` | |
| `web.resources` | 10m/32Mi, limit 128Mi | |
| `web.livenessProbe`, `web.readinessProbe` | `periodSeconds` 20 / 10 | Merged onto a fixed `httpGet` of `/web/index.html` |
| `web.service.*` | `ClusterIP`, `80` | |

### Media

| Value | Default | Description |
|---|---|---|
| `media.mountPath` | `/media` | The same in every workload, by construction: the API hands ffmpeg the path stored on the item row, so the media has to resolve to the same path in every pod that reads it |
| `media.volume` | `{}` | Any volume source — `persistentVolumeClaim`, `nfs`, `hostPath`. Empty mounts an `emptyDir`, which is a library with nothing in it |

```yaml
media:
  volume:
    persistentVolumeClaim:
      claimName: media
```

Whatever it is has to be readable from every node the pods land on. A
`ReadWriteOnce` claim binds to one node and the streaming pods scheduled
elsewhere get nothing, which surfaces as ffmpeg failing to open a path the API
reads perfectly well.

### Migrations

| Value | Default | Description |
|---|---|---|
| `migration.enabled` | `false` | Runs `gojellyfin migrate` as a `pre-install,pre-upgrade` hook Job. A release that carries a migration defaults it to true |
| `migration.backoffLimit` | `3` | |
| `migration.ttlSecondsAfterFinished` | `3600` | |
| `migration.resources` | 100m/128Mi, limits 500m/256Mi | |
| `migration.podAnnotations`, `.podSecurityContext`, `.securityContext`, `.nodeSelector`, `.tolerations`, `.affinity` | | As the workloads |

Nothing migrates at startup. The image's `entrypoint.sh` will migrate when
`MIGRATE_ON_START=true`, and the chart does not offer it: every replica would
race the same apply on every rollout, which is unsafe as soon as there is more
than one. The choice is between two safe options.

**The hook, `migration.enabled=true`.** One Job per release, before the pods
roll, and the upgrade fails without rolling anything if it fails. It costs the
rollout waiting on the Job, and it puts the migration on Helm's clock: a
migration that outlives `--timeout` fails the release while it is still running.

**A one-off run.** Nothing about the release migrates and you apply the schema
when you decide to, which is what you want when a migration is long or has to
land before the new image does:

```sh
kubectl run gojellyfin-migrate --rm --attach --restart Never \
  --image ghcr.io/freekingdean/gojellyfin:latest \
  --env DATABASE_URL="$DATABASE_URL" -- migrate
```

Migrations are safe to re-run either way — atlas records what it has applied in
`atlas_schema_revisions` and verifies `atlas.sum` on every run — so a retry
costs nothing. The Job and the pods must run the same image tag, and the chart
gives them the same one.

Some upgrades are `migrate` **plus a scan**, because the migration only stands a
value in and the scan is what writes the real one. The scan runs on the worker,
which is off by default here, so an upgrade with `migration.enabled=true` and no
worker leaves that half undone. The release that moved probe state onto the
media source (#551) is one of these, and its first scan re-probes every file
rather than skipping the unchanged ones: `probed_at` is NULL on every source the
migration created, and that is exactly what "needs probing" means. It is a
one-off, but it is ffprobe over the whole library on a worker whose defaults are
sized for steady state — give `worker.resources` more cpu for that run, or leave
it and let the run take longer.

### Routing

| Value | Default | Description |
|---|---|---|
| `httpRoute.enabled` | `false` | Gateway API `HTTPRoute`s. Off by default: a `parentRef` names a Gateway this chart cannot guess |
| `httpRoute.parentRefs` | `[]` | Required when enabled |
| `httpRoute.annotations` | `{}` | Applied to every route |

The hostname comes from the top-level `hostname`, so the routes match the name
the server advertises rather than a second copy of it. Enabled, this emits one
route per component sharing those parents: `/Videos` and `/Audio` to the
streaming pods, `/web` to the client, and `/` to the API. A longer prefix
outranks a shorter one, so the routes need no ordering between them, and the
API's cannot be narrowed because it owns paths at the root.

```yaml
hostname: media.example.com
httpRoute:
  enabled: true
  parentRefs:
    - name: my-gateway
      namespace: gateway-system
```

There is no Ingress template: an Ingress needs a controller and a class this
repository cannot guess, and the routing split the streaming pods depend on is
already expressed in Gateway API terms.

## Not in the chart

No ServiceAccount is created — nothing here talks to the Kubernetes API. No
PersistentVolumeClaim: the media is something you already have. No
HorizontalPodAutoscaler and no PodDisruptionBudget; scale the streaming pods by
hand for now, since a busy pod refusing with a 503 the gateway retries is what
absorbs load between them.
