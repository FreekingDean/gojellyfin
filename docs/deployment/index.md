# Deployment

gojellyfin is currently documented around its Kubernetes Helm chart. The chart
uses one application image for the API, streaming workload, worker and database
migration job, with a separate web-client workload.

Start with [Deploy with Helm](helm.md). It covers prerequisites, Secrets,
media mounts, routing, migrations and upgrades.

For the full list of chart values — including resource requests, autoscaling,
transcoder capacity and optional workloads — use the
[chart README](https://github.com/FreekingDean/gojellyfin/blob/main/charts/gojellyfin/README.md).
