# gojellyfin

A Helm chart for [gojellyfin](https://github.com/FreekingDean/gojellyfin), a Go
reimplementation of a Jellyfin media server.

## Install

The chart never carries a credential. `DATABASE_URL` is required — the binary
has no default and fails at start without one — so create the Secret first:

```sh
kubectl create secret generic gojellyfin \
  --from-literal DATABASE_URL='postgres://user:password@host:5432/gojellyfin?sslmode=require'

helm install gojellyfin ./charts/gojellyfin
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
| `transcode` | `server` | yes | The same command, routed the streaming paths |
| `worker` | `worker` | no | Temporal workflows — the library scan |
| `web` | — | yes | nginx serving `jellyfin-web`, copied out of the all-in-one image by an init container |

The transcode pods are not a second binary or a second protocol. They are the
same `server` command, and what makes them the transcoders is the HTTPRoute:
`/Videos` and `/Audio` go to them, everything else to the API. The pod that
serves the stream is the pod that runs ffmpeg, so no encode crosses the network
twice. Disable `transcode` and the API serves those paths itself.

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
| `httpPort` | `HTTP_PORT` | `8081` | What the server listens on. The container port, the Service target and the probes follow it |
| `database.existingSecret` | — | `""` | Secret holding the URL. Empty means the release's own fullname |
| `database.secretKey` | — | `DATABASE_URL` | Key within that Secret |
| `publishedServerURL` | `PUBLISHED_SERVER_URL` | `""` | Empty advertises no address at all. A client prefers `LocalAddress` when it believes it shares a network with the server, so naming one it cannot reach sends it nowhere |
| `corsOrigins` | `CORS_ORIGINS` | `[]` | Joined with commas. Empty reflects whatever origin asks; a literal `*` is not equivalent, because browsers reject it on credentialed requests |
| `temporal.hostPort` | `TEMPORAL_HOSTPORT` | `""` | Empty turns background work off and the server still starts |
| `temporal.namespace` | `TEMPORAL_NAMESPACE` | `""` | Required once `hostPort` is set; the binary refuses to invent one, and so does the chart |
| `api.transcoderJobs`, `transcode.transcoderJobs` | `TRANSCODER_JOBS` | `1`, `2` | Concurrent encodes on one pod |
| `api.transcoderStallTimeout`, `transcode.transcoderStallTimeout` | `TRANSCODER_STALL_TIMEOUT` | `30s` | How long a write to a client that vanished without closing may block before the encode is killed |

`MIGRATE_ON_START` is deliberately not exposed — see [Migrations](#migrations).

### TRANSCODER_JOBS and the cpu limit

One encode saturates about a core, so a pod that accepts more than it can run
makes every stream on it slower without finishing any sooner. Past its limit the
pod answers 503 with a `Retry-After` and the gateway retries against another
pod, and that refusal is the whole of the load balancing — there is no pool and
no shared state.

So `transcoderJobs` must stay at or below the workload's cpu limit, and the
chart refuses to render when it does not:

```
api.transcoderJobs is 4 with a cpu limit of 1: one encode saturates about a core…
```

Raise `resources.limits.cpu` and `transcoderJobs` together, or neither. Leaving
the variable unset is worse than either: the binary then falls back to the
number of cores on the *node*, which is not what the pod may use, so the chart
always sets it.

### Workloads

Each of `api`, `transcode`, `worker` and `web` takes the same shape:
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
| `transcode.enabled` | `true` | Disabled, the API serves `/Videos` and `/Audio` itself |
| `transcode.replicaCount` | `3` | |
| `transcode.resources` | 2 cpu/512Mi, limits 2 cpu/2Gi | Matched to `transcoderJobs: 2` |
| `transcode.mediaReadOnly` | `true` | Nothing here deletes a file |
| `transcode.livenessProbe` | period 20, timeout 5, 6 failures | Looser than the API's: a pod at capacity is busy, not broken |
| `transcode.readinessProbe`, `transcode.service.*` | as the API's | |
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

The API hands ffmpeg the path stored on the item row, so the media has to
resolve to the same path in every pod that reads it.

| Value | Default | Description |
|---|---|---|
| `media.mountPath` | `/media` | The same in every workload, by construction |
| `media.existingClaim` | `""` | Name a claim you manage and the chart creates none |
| `media.persistence.accessModes` | `[ReadWriteMany]` | A requirement, not a preference: a `ReadWriteOnce` claim binds to one node and the transcode pods scheduled elsewhere get nothing, which surfaces as ffmpeg failing to open a path the API reads perfectly well |
| `media.persistence.storageClassName` | `""` | Name a `ReadWriteMany` class (NFS, CephFS, Filestore). The cluster default is usually `ReadWriteOnce` block storage and binds wrong |
| `media.persistence.size` | `1Ti` | |
| `media.persistence.annotations` | `{}` | The claim also carries `helm.sh/resource-policy: keep`, so `helm uninstall` does not take the media with it |

### Migrations

| Value | Default | Description |
|---|---|---|
| `migration.enabled` | `false` | Runs `gojellyfin migrate` as a `pre-install,pre-upgrade` hook Job |
| `migration.backoffLimit` | `3` | |
| `migration.ttlSecondsAfterFinished` | `3600` | |
| `migration.resources` | 100m/128Mi, limits 500m/256Mi | |
| `migration.podAnnotations`, `.podSecurityContext`, `.securityContext`, `.nodeSelector`, `.tolerations`, `.affinity` | | As the workloads |

Nothing migrates at startup. The image's `entrypoint.sh` will migrate when
`MIGRATE_ON_START=true`, and the chart does not offer it: every replica would
race the same apply on every rollout, which is unsafe as soon as there is more
than one. The choice is between two safe options.

**A one-off run, which is the default.** Nothing about the release migrates, and
you apply the schema when you decide to:

```sh
kubectl run gojellyfin-migrate --rm --attach --restart Never \
  --image ghcr.io/freekingdean/gojellyfin:latest \
  --env DATABASE_URL="$DATABASE_URL" -- migrate
```

This is the one that keeps the schema change and the rollout separate, which is
what you want when a migration is long, when it needs to land before the new
image does, or when you want to watch it.

**The hook, `migration.enabled=true`.** One Job per release, before the pods
roll, and the upgrade fails without rolling anything if it fails. It costs the
rollout waiting on the Job, and it puts the migration on Helm's clock: a
migration that outlives `--timeout` fails the release while it is still running.

Migrations are safe to re-run either way — atlas records what it has applied in
`atlas_schema_revisions` and verifies `atlas.sum` on every run — so a retry
costs nothing. The Job and the pods must run the same image tag, and the chart
gives them the same one.

### Routing

| Value | Default | Description |
|---|---|---|
| `httpRoute.enabled` | `false` | Gateway API `HTTPRoute`s. Off by default: a `parentRef` names a Gateway this chart cannot guess |
| `httpRoute.parentRefs` | `[]` | Required when enabled |
| `httpRoute.hostnames` | `[]` | |
| `httpRoute.annotations` | `{}` | Applied to every route |

Enabled, it emits one route per component sharing those parents and hostnames:
`/Videos` and `/Audio` to the transcode pods, `/web` to the client, and `/` to
the API. A longer prefix outranks a shorter one, so the routes need no ordering
between them, and the API's cannot be narrowed because it owns paths at the
root.

```yaml
httpRoute:
  enabled: true
  parentRefs:
    - name: my-gateway
      namespace: gateway-system
  hostnames:
    - media.example.com
```

There is no Ingress template: an Ingress needs a controller and a class this
repository cannot guess, and the routing split the transcode pods depend on is
already expressed in Gateway API terms.

## Not in the chart

No ServiceAccount is created — nothing here talks to the Kubernetes API. No
HorizontalPodAutoscaler and no PodDisruptionBudget; scale the transcode pods by
hand for now, since a busy pod refusing with a 503 the gateway retries is what
absorbs load between them.
