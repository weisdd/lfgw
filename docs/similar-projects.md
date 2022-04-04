# Similar projects

Please note, this part of documentation was written around March 2021. Things might have changed since then.

## Prometheus filter proxy

Link: [Prometheus filter proxy](https://github.com/hoffie/prometheus-filter-proxy)

Minuses:

- based on Prometheus library, so might not support some of Victoria Metrics' extensions;
- does not support autoconfiguration;
- it's an overly simple implementation that relies only on URI paths to define the scope of available metrics, so a user might potentially get access to any metrics should the URL become exposed;
- it's based on http client, thus unlikely to be ready for high volumes of requests;
- does not allow to filter out requests to sensitive endpoints (like `/admin/tsdb`);
- no longer maintained.

## Prometheus ACLs

Link: [Prometheus ACLs](https://github.com/bitsbeats/prometheus-acls)

Pluses:

- based on `httputil` reverse proxy;
- supports label filter deduplication;
- extensive ACL syntax, which allows to specify any label filters, not necessarily limit to `namespace`;
- has an OIDC callback and `gorilla/sessions`, so it's possible to obtain a token through the application itself;
- validates jwt-token;
- metrics;
- configurable JMESPath.

Minuses:

- a user cannot have multiple roles (`When you have multiple roles, the first one that is mentioned in prometheus-acls will be used.`);
- does not support autoconfiguration;
- based on Prometheus library, so might not support some of Victoria Metrics' extensions;
- does not allow to filter out requests to sensitive endpoints (like `/admin/tsdb`);
- does not rewrite requests to `/federate` endpoint (at least, at the time of writing);
- http server time-outs are not configured, thus might retain http sessions for much longer than needed.
