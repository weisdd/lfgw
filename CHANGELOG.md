# CHANGELOG

## 0.11.2

- Key changes:
  - Non-GET / -POST requests are now blocked only when safe mode is enabled.

## 0.11.1

- Key changes:
  - Added an option to skip the file with predefined roles through setting `ACL_PATH` to an empty value. This might be useful in environments that fully rely on Assumed Roles (=autoconfiguration).

## 0.11.0

- Key changes:
  - Automatically set `GOMAXPROCS` to match Linux container CPU quota via [uber-go/automaxprocs](https://github.com/uber-go/automaxprocs). Enabled by default, can be turned off via `SET_GOMAXPROCS: false`.

## 0.10.0

- Key changes:
  - Added support for autoconfiguration through Assumed roles (disabled by default, can be enabled through `ASSUMED_ROLES: true`):
    - In environments, where OIDC-role names match names of namespaces, ACLs can be constructed on the fly (e.g. `["role1", "role2"]` will give access to metrics from namespaces `role1` and `role2`; `kube.*` - to namespaces starting with `kube.*`, `.*` - to all metrics). The roles specified in `acl.yaml` are still considered and get merged with assumed roles;
    - Thanks to [@aberestyak](https://github.com/aberestyak/) for the idea;
  - Logs:
    - Log OIDC roles when debug is enabled. The field will contain all roles present in the token, not only those that are considered during ACL generation process.

## 0.9.0

- Key changes:
  - Added support for deduplication (enabled by default, can be turned off through `ENABLE_DEDUPLICATION: false`):
    - Previously, a label filter with a positive regexp was always added or replaced if a user had a regexp policy;
    - When deduplication is enabled, these queries will stay unmodified:
      - `min.*, stolon`, query: `request_duration{namespace="minio"}` - a non-regexp label filter that matches policy;
      - `min.*, stolon`, query: `request_duration{namespace=~"minio"}` - a "fake" regexp (no special symbols) label filter that matches policy;
      - `min.*, stolon`, query: `request_duration{namespace=~"min.*"}` - a label filter is a subfilter of the policy;
  - ACLs:
    - ACLs containing one word regexp expressions will have their anchors stripped;
    - Anchors are no longer added to complex ACLs, because Prometheus always treats regex expressions as fully anchored;
    - Fix: if a user had multiple roles, and one of the roles contained `.*` amongst other entries, getLF would pass all roles to PrepareLF instead of directly returning a full access role. It didn't cause any security issues as PrepareLF would still return a full access label filter, it just made the process lengthier;
  - Logs:
    - GET and POST queries are now logged in unescaped form, so it gets easier for a reader to compare original and modified requests;
    - duration is now logged without unit suffix, time is represented in seconds;
  - Bugfixes:
    - admin POST-requests failed to get proxied to upstream, because logging middleware was not updating Content-Length after reading PostForm. The issue was introduced in 0.7.0;
  - Added more tests.

## 0.8.0

- Key changes:
  - Exposed runtime metrics.

## 0.7.0

- Key changes:
  - Added some tests;
  - Moved to Go 1.18, Alpine 3.15.1;
  - Moved to zerolog:
    - Pretty formatting by default, JSON is also an option (env: `LOG_FORMAT`: `pretty`, `json`);
    - Optional access log (env `LOG_REQUESTS`: `true`);
    - NOTE: Logging format is subject to change.

## 0.6.0

- Key changes:
  - Added a graceful shutdown mechanism with a configurable timeout.

## 0.5.0

- Key changes:
  - Added support for [automatic expression optimizations](https://pkg.go.dev/github.com/VictoriaMetrics/metricsql#Optimize) for non-full access requests;
- Minor changes:
  - lfgw:
    - Slight improvements in code style;
    - Migrated to go 1.17;
    - Fully deprecated non-OIDC modes;
    - Bumped go.mod deps;
    - Updated base images;
    - Enabled more linters for .golangci-lint;
  - CI:
    - Simplified Taskfile;
    - Enabled dependabot alerts;
    - Added a workflow to publish docker images.

## 0.4.0

- Added support for multiple roles (previously, only the first one would be picked).

## 0.3.0

- Added support for POST requests;
- Updated metricsql from `v0.10.1` to `v0.14.0`.

## 0.2.3

- Added `/federate` to a list of requests that should be rewritten.

## 0.2.2

- Moved to go 1.16;
- Bumped dependencies;
- Improved build caching.

## 0.2.1

- Adjusted request rewrite logic, so now all requests containing `/api/` are rewritten, whereas previously only those starting with `/api/`. So, non-standard URIs are taken into account now.
- Explicitly specified flush interval for reverse proxy;

## 0.2.0

- Added support for extra authorization headers (X-Forwarded-Access-Token, X-Auth-Request-Access-Token).

## 0.1.1

- Bugfix for doubling URI-path while proxying in case UPSTREAM_URL has non-empty URI.

## 0.1.0

- Initial release.
