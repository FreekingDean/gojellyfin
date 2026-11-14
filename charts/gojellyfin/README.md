# gojellyfin

A Helm chart for [gojellyfin](https://github.com/FreekingDean/gojellyfin), a Go
reimplementation of a Jellyfin media server.

## Install

The chart never carries a credential. `DATABASE_URL` is required — the binary
has no default and fails at start without one — so create the Secret first:

```sh
kubectl create secret generic gojellyfin \
  --from-literal DATABASE_URL='postgres://user:password@host:5432/gojellyfin?sslmode=require'

helm install gojellyfin oci://ghcr.io/freekingdean/charts/gojellyfin \
  --version 0.0.1 \
  --set hostname=media.example.com \
  --set-json 'media.volume={"persistentVolumeClaim":{"claimName":"media"}}'
```

The release name decides the Secret's default name: an install named `gojellyfin`
looks for a Secret named `gojellyfin`, and `database.existingSecret` names a
different one. Nothing in `values.yaml` ever holds the URL.

Every merge to `main` is a release, and each publishes the chart at the same
version as the image it deploys, so `--version` pins both: an upgrade is
`helm upgrade --version <next>`, and a rollback is a rollback to a build rather
than to whatever `latest` meant that day. The versions are the image's, without
the git tag's `v`. Installing from a checkout of `charts/gojellyfin` instead
gets `latest`, because an unreleased chart has no version to name; pin
`image.tag` when you do.

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

The chart aims to be configured in the concepts an operator has — a hostname, a
volume the media lives on, how many replicas, how much cpu — rather than in
Kubernetes fields it would then own forever. Everything a pod needs that is not
one of those lives under `pod`, in one generic block rather than twenty named
values.

### The smallest install

```yaml
hostname: media.example.com
database:
  existingSecret: gojellyfin-db
media:
  volume:
    persistentVolumeClaim:
      claimName: media
httpRoute:
  enabled: true
  parentRefs:
    - name: my-gateway
      namespace: gateway-system
```

Four things, and only the first two are strictly required. `database.existingSecret`
can be dropped if the Secret is named after the release. `media.volume` can be
dropped, and you get an `emptyDir` — a library with nothing in it. `httpRoute`
can be dropped if something other than Gateway API fronts the Services. Nothing
else has to be set for a working install.

### Image and naming

| Value | Default | Description |
|---|---|---|
| `nameOverride` | `""` | Replaces the chart name in resource names and labels |
| `fullnameOverride` | `""` | Replaces the whole generated name |
| `image.repository` | `ghcr.io/freekingdean/gojellyfin` | The one image every gojellyfin workload runs |
| `image.tag` | `""` | Empty takes the chart's `appVersion`, which a published chart sets to its own version and a checkout leaves at `latest`. The API, the streaming pods, the worker and the migration Job all use it — the migrations are embedded in the binary, so two tags mean the schema one image applied and the schema the other expects can differ. Which build is actually running is stamped into the binary at image build time and answered as `SystemInfo.PackageName` |
| `image.pullPolicy` | `""` | Empty is `Always` for a tag that moves and `IfNotPresent` for one that does not. A node that has already pulled `latest` would otherwise keep serving that build across every rollout. `Always` does not make `latest` safe: a `helm upgrade` that renders the same tag changes no pod spec and rolls nothing out, which is why a release pins a version |

### Server environment

Every variable below is read by `internal/env`, which is the only package that
reads the environment.

| Value | Environment variable | Default | Description |
|---|---|---|---|
| `hostname` | `PUBLISHED_SERVER_URL` | `""` | The name clients reach the server by. The routes match it, and the public endpoints advertise `<scheme>://<hostname>`. Empty advertises no address at all — a client prefers `LocalAddress` when it believes it shares a network with the server, so naming one it cannot reach sends it nowhere |
| `scheme` | | `https` | What the gateway in front of this listens on, and the only thing it is used for. Advertising `https` over a plain listener loses a client exactly as advertising a wrong hostname does |
| `httpPort` | `HTTP_PORT` | `8081` | What the server listens on. The container port, the Service target and the probes follow it |
| `env` | anything | `[]` | Extra container env for the API, the streaming pods and the worker, in Kubernetes' own `env` shape. It is rendered first, so another value can refer to one of these as `$(NAME)` — `tracing.otlpEndpoint: http://$(HOST_IP):4318` with `HOST_IP` read from `status.hostIP` reaches a node-local collector's OTLP/HTTP receiver, which is what the exporter speaks |
| `database.existingSecret` | `DATABASE_URL` | `""` | Secret holding the URL. Empty means the release's own fullname |
| `database.secretKey` | — | `DATABASE_URL` | Key within that Secret |
| `tmdb.existingSecret` | `TMDB_API_KEY` | `""` | Secret holding the TMDB key. Empty turns metadata lookups off, so items keep their source's titles and get no artwork |
| `tmdb.secretKey` | — | `TMDB_API_KEY` | Key within that Secret |
| `sources.existingSecret` | `SOURCE_API_KEY_*` | `""` | Secret whose keys are the variable names Sonarr and Radarr sources are configured with, loaded into the API and the worker. Empty leaves every source failing its test and its scan |
| `cors.enabled` | `CORS_ORIGINS` | `false` | Off reflects whatever origin asks; a literal `*` is not equivalent, because browsers reject it on credentialed requests |
| `cors.origin` | | `""` | The one origin allowed when enabled. Enabling without one refuses to render: an empty value reflects every origin, so the restriction would restrict nothing |
| `tracing.otlpEndpoint` | `OTEL_EXPORTER_OTLP_ENDPOINT` | `""` | Empty exports no traces |
| `media.directories` | `MEDIA_DIRECTORIES` | `[]` | The roots the library browser offers; empty follows `media.mountPath` and every `media.extraVolumes` path |
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

| Value | Default | Description |
|---|---|---|
| `api.replicas` | `2` | The API holds no per-request state and needs no session affinity |
| `api.resources` | 200m/256Mi, limits 1 cpu/1Gi | Also decides the encode slots, above |
| `streaming.enabled` | `true` | Disabled, the API serves `/Videos` and `/Audio` itself |
| `streaming.replicas` | `3` | |
| `streaming.resources` | 2 cpu/512Mi, limits 2 cpu/2Gi | |
| `worker.enabled` | `false` | Needs `temporal.hostPort`; the chart refuses to render an enabled worker with nothing to dial, because it would only crash loop |
| `worker.replicas` | `1` | Left off the Deployment while `worker.autoscaling.enabled`, so an upgrade does not reset what the autoscaler chose |
| `worker.resources` | 200m/256Mi, limits 1 cpu/1Gi | The cpu limit is also the worker's slots: it runs `GOMAXPROCS` activities per task queue, and Go reads that off the limit |
| `worker.stopTimeoutSeconds` | `300` | `TEMPORAL_WORKER_STOP_TIMEOUT`. On SIGTERM the worker stops polling and gives running activities this long to finish; what is left is cancelled and retried on another worker. `terminationGracePeriodSeconds` follows it with 30s to spare unless `worker.pod` names one |
| `worker.autoscaling.enabled` | `false` | A cpu `HorizontalPodAutoscaler`. Needs `worker.resources.requests.cpu`, since utilization is measured against it, and metrics-server in the cluster |
| `worker.autoscaling.minReplicas`, `.maxReplicas` | `1`, `3` | |
| `worker.autoscaling.targetCPUUtilizationPercentage` | `80` | Of the request, not the limit |
| `worker.autoscaling.behavior` | one pod every 2m after 5m quiet | The HPA's `behavior`, passed through. The default scale-in is slow so a lull between two scans does not drain a worker the second one wants |
| `worker.podDisruptionBudget.enabled`, `.maxUnavailable` | `false`, `1` | What a drain, kured and the descheduler honour. The autoscaler scales in without evicting, so this does not slow it |
| `web.enabled` | `true` | |
| `web.replicas` | `2` | |
| `web.client.repository`, `.tag`, `.pullPolicy` | `jellyfin/jellyfin`, `10.10.0`, `""` | `jellyfin-web` is not published on its own — no image, no npm package, no built assets in its releases — so an init container copies it out of the all-in-one image into an emptyDir. Pin it to a released client rather than to the announced API version: the server says 12.0.0 out of the vendored spec, 12 has no released all-in-one image, and a client newer than what is implemented calls endpoints that answer 501. 10.10.0 is what `make e2e` drives |
| `web.client.resources` | 50m/64Mi, limit 256Mi | |
| `web.image.repository`, `.tag`, `.pullPolicy` | `nginxinc/nginx-unprivileged`, `1.31.7-alpine`, `""` | Pinned, because a floating tag is the same drift `latest` is. The unprivileged image listens on 8080 and owns the directories nginx writes to, which is what lets `runAsNonRoot` hold. The port is not a value: nothing here rewrites nginx's config, so it listens on 8080 whatever a value said |
| `web.resources` | 10m/32Mi, limit 128Mi | |

What the chart owns rather than exposing: the probes (`/System/Ping` is the only
endpoint that is both public and free of any database access, and the streaming
pods get a looser liveness because a pod at capacity is busy rather than broken),
and which workloads mount the media writable — the API does because deleting an
item is an API path, and nothing else has a reason to.

### Pods

`pod` is merged into every pod the chart renders, and each workload's own `pod`
overrides it key by key. Every key except `annotations`, `labels` and
`containerSecurityContext` is passed through to the pod spec verbatim, so
anything Kubernetes accepts there works without the chart naming it. A `false`
or a `0` is passed through too; only an empty value is left out.

| Value | Default | Description |
|---|---|---|
| `pod.annotations` | `{}` | On the pod template's metadata |
| `pod.labels` | `{}` | Added to the pod template's labels — a mesh opt-out like `istio.io/dataplane-mode: none`, say. The chart's selector labels win a collision, because a pod that stops matching its own Deployment's selector is orphaned rather than relabelled |
| `pod.automountServiceAccountToken` | `false` | Nothing here talks to the Kubernetes API, so no pod carries a token |
| `pod.containerSecurityContext` | no privilege escalation, drops ALL, `RuntimeDefault` seccomp | On each container, init containers included. No `runAsNonRoot` for the gojellyfin containers: the image's `USER` is a name rather than a uid and kubelet refuses to start a container whose non-rootness it cannot check numerically. The image drops privilege on its own. The web pod sets `runAsNonRoot` and uid 101, nginx's, at the pod level through `web.pod.securityContext`, so the init container copying the client out of the root-by-default `jellyfin/jellyfin` image runs unprivileged too |
| `pod.securityContext` | `{}` | Pod-level, passed through |
| `pod.imagePullSecrets` | `[]` | Passed through |
| `pod.nodeSelector`, `.tolerations`, `.affinity` | empty | Passed through, and named here only because they are the common ones |

```yaml
pod:
  nodeSelector:
    kubernetes.io/arch: amd64
streaming:
  pod:
    runtimeClassName: gvisor
    topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: kubernetes.io/hostname
        whenUnsatisfiable: ScheduleAnyway
        labelSelector:
          matchLabels:
            app.kubernetes.io/component: streaming
```

Neither `runtimeClassName` nor `topologySpreadConstraints` is a value this chart
declares. That is the point: the chart does not have to grow a value, and your
`values.yaml` does not have to wait for it to.

### Services and media

| Value | Default | Description |
|---|---|---|
| `service.type`, `.port`, `.annotations`, `.labels` | `ClusterIP`, `80`, `{}`, `{}` | Applied to all three Services. An Ingress needs a controller this chart cannot guess; `httpRoute` below is the supported edge |
| `media.mountPath` | `/media` | The same in every workload, by construction: the API hands ffmpeg the path stored on the item row, so the media has to resolve to the same path in every pod that reads it |
| `media.volume` | `{}` | Any volume source — `persistentVolumeClaim`, `nfs`, `hostPath`. Empty mounts an `emptyDir`, which is a library with nothing in it |
| `media.extraVolumes` | `[]` | More media beside the first, each `name`, `mountPath` and `volume`, mounted into every workload exactly as `media.volume` is — writable on the API, read-only elsewhere. A library whose files live on two claims needs both at the same path in every pod, for the same reason the first one does |

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
| `migration.resources` | 100m/128Mi, limits 500m/256Mi | The Job takes its pod plumbing from `pod`, and its retries and TTL from the chart |

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
IMAGE=ghcr.io/freekingdean/gojellyfin:0.0.1
kubectl run gojellyfin-migrate --rm --attach --restart Never --image "$IMAGE" \
  --overrides '{"spec":{"containers":[{"name":"gojellyfin-migrate","image":"'"$IMAGE"'","args":["migrate"],"env":[{"name":"DATABASE_URL","valueFrom":{"secretKeyRef":{"name":"gojellyfin","key":"DATABASE_URL"}}}]}]}}'
```

The URL is read out of the Secret by the kubelet rather than passed with
`--env`, which would write it in plain text into the pod spec. Use the tag the
release runs; the notes `helm install` prints carry this command with the tag
and the Secret filled in.

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

**`/web` is two things, and a naive prefix route breaks one of them.** It is
where the client is served, and it is also where two API endpoints live:
`/web/ConfigurationPage` and `/web/ConfigurationPages`. `jellyfin-web` asks for
them in both cases from different places: the dashboard menu fetches
`/web/ConfigurationPages`, My Plugins fetches `web/configurationpages`,
opening a page fetches `/web/configurationpage?name=…` and resource urls use
`web/ConfigurationPage`. A gateway that sends all of `/web` to the client has
nginx look for those files and 404, and the plugin pages disappear. The chart
carries an `Exact` rule for each of the four spellings back to the API service;
the server's own router does not care about case either. Two `Exact` rules on
the capitalised spellings alone let the lower-case page request through to
nginx, and a single case-insensitive `RegularExpression` rule did not work on a
real gateway: it is an extended feature whose support and precedence against
the `/web` prefix are implementation-specific. `Exact` is core, and the spec
gives an exact match precedence over any prefix across every route, so four
literal rules work on any conformant gateway. Writing your own gateway config
means carrying these four with it.

They are rendered whether or not `web.enabled` is set. With the client off they
are merely redundant with `/`, and they still point at the API, which this chart
always deploys; with a client served from outside the chart they are what keeps
the dashboard working.

Two properties of Gateway API path matching are worth knowing here. Matching is
**by whole path element**, so `/Videos` cannot catch a path like `/VideosFoo`.
And it is **case-sensitive**, while this server's own mux is deliberately not —
a client that spells a stream path in another case simply lands on the API pods,
which run the same binary and answer it, bounded by `api.transcoderJobs` rather
than by `streaming.transcoderJobs`. For the same reason, the non-stream paths
that live under those prefixes — `/Videos/{id}/Subtitles`, `/Audio/{id}/Lyrics`,
`/Videos/MergeVersions` — are served correctly by the streaming pods: they are
full API pods that happen to be the ones running ffmpeg.

There is no Ingress template: an Ingress needs a controller and a class this
repository cannot guess, and the routing split the streaming pods depend on is
already expressed in Gateway API terms.

## Not in the chart

No ServiceAccount is created — nothing here talks to the Kubernetes API. No
PersistentVolumeClaim: the media is something you already have. No
HorizontalPodAutoscaler and no PodDisruptionBudget; scale the streaming pods by
hand for now, since a busy pod refusing with a 503 the gateway retries is what
absorbs load between them.
