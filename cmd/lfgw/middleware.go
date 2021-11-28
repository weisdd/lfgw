package main

import (
	"context"
	"fmt"
	"io"
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

// prohibitedMethods forbids all the methods aside from "GET" and "POST".
func (app *application) prohibitedMethodsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			w.Header().Set("Allow", fmt.Sprintf("%s, %s", http.MethodGet, http.MethodPost))
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

		userRoles, err := app.getUserRoles(claims.Roles)
		if err != nil {
			app.errorLog.Printf("%s (%s, %s)", err, claims.Username, claims.Email)
			app.clientErrorMessage(w, http.StatusUnauthorized, err)
			return
		}

		lf, err := app.getLF(userRoles)
		if err != nil {
			app.errorLog.Printf("%s (%s, %s)", err, claims.Username, claims.Email)
			app.clientErrorMessage(w, http.StatusUnauthorized, err)
			return
		}

		hasFullaccess := app.HasFullaccessValue(lf.Value)

		ctx = context.WithValue(ctx, contextKeyHasFullaccess, hasFullaccess)
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

		hasFullaccess, ok := r.Context().Value(contextKeyHasFullaccess).(bool)
		if ok && hasFullaccess {
			app.debugLog.Print("Request is passed through")
			next.ServeHTTP(w, r)
			return
		}

		lf, ok := r.Context().Value(contextKeyLabelFilter).(metricsql.LabelFilter)
		if !ok {
			app.serverError(w, fmt.Errorf("LF is not set in the context"))
			return
		}

		err := r.ParseForm()
		if err != nil {
			app.errorLog.Printf("%s", err)
			app.clientError(w, http.StatusBadRequest)
			return
		}

		// Adjust GET params
		getParams := r.URL.Query()
		newGetParams, err := app.prepareQueryParams(&getParams, lf)
		if err != nil {
			app.errorLog.Printf("%s", err)
			app.clientError(w, http.StatusBadRequest)
			return
		}
		r.URL.RawQuery = newGetParams

		// Adjust POST params
		// Partially inspired by https://github.com/bitsbeats/prometheus-acls/blob/master/internal/labeler/middleware.go
		if r.Method == http.MethodPost {
			newPostParams, err := app.prepareQueryParams(&r.PostForm, lf)
			if err != nil {
				app.errorLog.Printf("%s", err)
				app.clientError(w, http.StatusBadRequest)
				return
			}
			newBody := strings.NewReader(newPostParams)
			r.ContentLength = newBody.Size()
			r.Body = io.NopCloser(newBody)
		}

		next.ServeHTTP(w, r)
	})
}
