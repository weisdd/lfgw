package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/metricsql"
)

// healthzMiddleware is a workaround to support healthz endpoint while forwarding everything else to an upstream.
func (app *application) healthzMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/healthz") {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// prohibitedMethods forbids all the methods aside from "GET".
func (app *application) prohibitedMethodsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			app.clientError(w, http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// prohibitedPaths forbids access to some destinations that should not be proxied.
func (app *application) prohibitedPathsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.SafeMode {
			if strings.Contains(r.URL.Path, "/admin/tsdb") || strings.Contains(r.URL.Path, "/api/v1/write") {
				app.errorLog.Printf("Blocked a request to %s", r.URL.Path)
				app.clientError(w, http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// proxyHeadersMiddleware sets proxy headers.
func (app *application) proxyHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.SetProxyHeaders {
			r.Header.Set("X-Forwarded-For", r.RemoteAddr)
			r.Header.Set("X-Forwarded-Proto", r.URL.Scheme)
			r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
		}
		next.ServeHTTP(w, r)
	})
}

// oidcModeMiddleware verifies a jwt token, and, if valid and authorized,
// adds a respective label filter to the request context.
func (app *application) oidcModeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawAccessToken, err := app.getRawAccessToken(r)
		if err != nil {
			app.errorLog.Println(err)
			app.clientErrorMessage(w, http.StatusUnauthorized, err)
			return
		}

		ctx := r.Context()
		accessToken, err := app.verifier.Verify(ctx, rawAccessToken)
		if err != nil {
			app.errorLog.Println(err)
			app.clientErrorMessage(w, http.StatusUnauthorized, err)
			return
		}

		// Extract custom claims
		var claims struct {
			Roles    []string `json:"roles"`
			Username string   `json:"preferred_username"`
			Email    string   `json:"email"`
		}
		if err := accessToken.Claims(&claims); err != nil {
			app.errorLog.Println(err)
			app.clientErrorMessage(w, http.StatusUnauthorized, err)
			return
		}

		userRole, err := app.getUserRole(claims.Roles)
		if err != nil {
			app.errorLog.Printf("%s (%s, %s)", err, claims.Username, claims.Email)
			app.clientErrorMessage(w, http.StatusUnauthorized, err)
			return
		}

		hasFullaccessRole := app.hasFullaccessRole(userRole)
		lf := app.getLF(userRole)

		ctx = context.WithValue(ctx, contextKeyHasFullaccessRole, hasFullaccessRole)
		ctx = context.WithValue(ctx, contextKeyLabelFilter, lf)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// rewriteRequestMiddleware rewrites a request before forwarding it to the upstream.
func (app *application) rewriteRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Rewrite request destination
		r.Host = app.UpstreamURL.Host

		if !strings.Contains(r.URL.Path, "/api/") && !strings.Contains(r.URL.Path, "/federate") {
			app.debugLog.Print("Not an API request, passing through")
			next.ServeHTTP(w, r)
			return
		}

		// Since the value is false by default, we don't really care if it exists in the context
		hasFullaccessRole, _ := r.Context().Value(contextKeyHasFullaccessRole).(bool)
		if hasFullaccessRole {
			app.debugLog.Print("Request is passed through")
			next.ServeHTTP(w, r)
			return
		}

		lf, ok := r.Context().Value(contextKeyLabelFilter).(metricsql.LabelFilter)
		if !ok {
			app.serverError(w, fmt.Errorf("LF is not set in the context"))
			return
		}

		rawQueryParams, err := app.prepareRawQueryParams(r, lf)
		if err != nil {
			app.errorLog.Printf("%s", err)
			app.clientError(w, http.StatusBadRequest)
			return
		}

		r.URL.RawQuery = rawQueryParams

		next.ServeHTTP(w, r)
	})
}
