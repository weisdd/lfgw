# LFGW

lfgw is a trivial reverse proxy based on `httputil` and `VictoriaMetrics/metricsql` with a purpose of dynamically rewriting requests to Prometheus-like backends.

More specifically, it manipulates label filters in metric expressions to reduce the scope of metrics exposed to an end user.

## Key functions

* non-opinionated proxying of requests with valid jwt tokens - there's Prometheus to decide whether request is valid or not;
* ACL-based request rewrites with implicit deny;
* supports Victoria Metrics' PromQL extensions;
* since it's based on `VictoriaMetrics/metricsql` library, which has way simpler interface than `prometheus`, there is no need to write a separate implementation for every type of MetricExpr on the planet;
* it's based on the middleware pattern, so it's easy to implement other, non-OIDC, modes should the need be;
* support for different headers with access tokens (X-Forwarded-Access-Token, X-Auth-Request-Access-Token, Authorization);
* requests to sensitive endpoints are blocked by default;
* requests to both `/api/*` and `/federate` endpoints are protected (=rewritten).

## Current limitations

* there's no OIDC callback, so it can only proxy requests that already have a jwt token;
* other modes (POSITIVE_REGEXP, NEGATIVE_REGEXP, NAMESPACE) are deprecated.

## TODO

* tests for handlers;
* improve naming;
* log slow requests;
* metrics;
* add CLI interface (currently, only environment variables are used);
* configurable JMESPath for the `roles` attribute;
* OIDC callback to support for proxying Prometheus web-interface itself;
* structured logging (it'll require an intermediate interface for logging `httputil`'s error logs);
* simple deduplication if there is any performance issue (another option: use `trickster` for request optimizations).

## Similar projects

### Prometheus filter proxy

Link: [Prometheus filter proxy](https://github.com/hoffie/prometheus-filter-proxy)

Minuses:

* based on Prometheus library, so might not support some of Victoria Metrics' extensions;
* it's an overly simple implementation that relies only on URI paths to define the scope of available metrics, so a user might potentially get access to any metrics should the URL become exposed;
* it's based on http client, thus unlikely to be ready for high volumes of requests;
* does not allow to filter out requests to sensitive endpoints (like `/admin/tsdb`);
* no longer maintained.

### Prometheus ACLs

Link: [Prometheus ACLs](https://github.com/bitsbeats/prometheus-acls)

Pluses:

* based on `httputil` reverse proxy;
* supports label filter deduplication;
* extensive ACL syntax, which allows to specify any label filters, not necessarily limit to `namespace`;
* has an OIDC callback and `gorilla/sessions`, so it's possible to obtain a token through the application itself;
* validates jwt-token;
* metrics;
* configurable JMESPath.

Minuses:

* based on Prometheus library, so might not support some of Victoria Metrics' extensions;
* does not allow to filter out requests to sensitive endpoints (like `/admin/tsdb`);
* does not rewrite requests to `/federate` endpoint (at least, at the time of writing);
* http server time-outs are not configured, thus might retain http sessions for much longer than needed.

## Configuration

OIDC roles are expected to be present in `roles` within a jwt token.

### Environment variables

| Module               | Variable            | Default Value | Description                                                  |
| -------------------- | ------------------- | ------------- | ------------------------------------------------------------ |
| **General settings** |                     |               |                                                              |
|                      | `LFGW_MODE`         | `oidc`        | Currently, only `oidc` mode is supported. So, never mind.    |
|                      |                     |               |                                                              |
| **Logging**          |                     |               |                                                              |
|                      | `DEBUG`             | `false`       | Whether to print out debug log messages.                     |
|                      |                     |               |                                                              |
| **HTTP Server**      |                     |               |                                                              |
|                      | `PORT`              | `8080`        | Port the web server will listen on.                          |
|                      | `READ_TIMEOUT`      | `10s`         | `ReadTimeout` covers the time from when the connection is accepted to when the request body is fully read (if you do read the body, otherwise to the end of the headers). [More details](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) |
|                      | `WRITE_TIMEOUT`     | `10s`         | `WriteTimeout` normally covers the time from the end of the request header read to the end of the response write (a.k.a. the lifetime of the ServeHTTP). [More details](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) |
|                      |                     |               |                                                              |
| **Proxy**            |                     |               |                                                              |
|                      | `UPSTREAM_URL`      |               | Prometheus URL, e.g. `http://prometheus.microk8s.localhost`. |
|                      | `SAFE_MODE`         | `true`        | Whether to block requests to sensitive endpoints like `/api/v1/admin/tsdb`, `/api/v1/insert`. |
|                      | `SET_PROXY_HEADERS` | `false`       | Whether to set proxy headers (`X-Forwarded-For`, `X-Forwarded-Proto`, `X-Forwarded-Host`). |
|                      |                     |               |                                                              |
| **OIDC**             |                     |               |                                                              |
|                      | `ACL_PATH`          | `./acl.yaml`  | Path to a file with ACL definitions (OIDC role to namespace bindings). |
|                      | `OIDC_REALM_URL`    |               | OIDC Realm URL, e.g. `https://auth.microk8s.localhost/auth/realms/cicd` |
|                      | `OIDC_CLIENT_ID`    |               | OIDC Client ID (1*)                                          |

(1*): since it's grafana who obtains jwt-tokens in the first place, the specified client id must also be present in the forwarded token (the `audience` field). To put it simply, better to use the same client id for both grafana and lfgw.

### acl.yaml syntax

The file with ACL definitions has a simple structure:

```yaml
role: namespace, namespace2
```

For example:

```yaml
team0: .*              # all metrics
team1: minio           # only those with namespace="minio"
team2: min.*           # only those matching namespace=~"min.*"
team3: minio, stolon   # only those matching namespace=~"^(minio|stolon)$"
team4: min.*, stolon   # only those matching namespace=~"^(min.*|stolon)$"
```

To summarize, here are the key principles used for rewriting requests:

* `.*` - all requests are simply forwarded to an upstream;

* `minio` - all label filters with the `namespace` label are removed, then `namespace="minio"` is added;
* `min.*` -  positive regex-match label filters (`namespace=~"X"`) are removed, then `namespace=~"mi.*"` is added;
* `minio, stolon` - positive regex-match label filters (`namespace=~"X"`) are removed, then `namespace=~"^(minio|stolon)$"` is added;
* `min.*, stolon` - positive regex-match label filters (`namespace=~"X"`) are removed, then `namespace=~"^(min.*|stolon)$"` is added.
