# Networking and reverse proxies

Set the public hostname to the address users and clients actually reach. With Helm, use `hostname`; the chart passes it to the server as `PUBLISHED_SERVER_URL` and uses it when creating Gateway API routes.

The advertised scheme must match the listener exposed by your gateway or proxy. If clients are told to use HTTPS while the public endpoint only serves plain HTTP, they will fail to connect.

## Gateway API

The chart can create HTTPRoutes when `httpRoute.enabled` is true and `httpRoute.parentRefs` names an existing Gateway. It splits traffic between the API, streaming workload and web client.

If you configure routing outside the chart, preserve the same split:

- `/Videos` and `/Audio` should reach the streaming service when it is enabled.
- `/web` serves the static client.
- Some API endpoints also live under `/web`, including `/web/ConfigurationPage` and `/web/ConfigurationPages`. Route those exact endpoints to the API rather than the static client.
- Everything else goes to the API.

## Trusted proxies

Set `trustedProxies` to the network ranges used by your trusted proxy or gateway. This allows the server to interpret forwarded client addresses correctly. Leave it empty if you do not want to trust forwarded headers.

Only trust networks controlled by you. Trusting arbitrary clients lets them supply forwarded addresses.

For all chart settings and routing details, see the [Helm chart reference](https://github.com/FreekingDean/gojellyfin/blob/main/charts/gojellyfin/README.md).
