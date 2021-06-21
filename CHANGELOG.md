# CHANGELOG

## 0.4.0

* Added support for multiple roles (previously, only the first one would be picked).

## 0.3.0

* Added support for POST requests;
* Updated metricsql from `v0.10.1` to `v0.14.0`.

## 0.2.3

* Added `/federate` to a list of requests that should be rewritten.

## 0.2.2

* Moved to go 1.16;
* Bumped dependencies;
* Improved build caching.

## 0.2.1

* Adjusted request rewrite logic, so now all requests containing `/api/` are rewritten, whereas previously only those starting with `/api/`. So, non-standard URIs are taken into account now.
* Explicitly specified flush interval for reverse proxy;

## 0.2.0

* Added support for extra authorization headers (X-Forwarded-Access-Token, X-Auth-Request-Access-Token).

## 0.1.1

* Bugfix for doubling URI-path while proxying in case UPSTREAM_URL has non-empty URI.

## 0.1.0

* Initial release.
