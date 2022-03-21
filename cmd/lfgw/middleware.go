package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/rs/zerolog/hlog"
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

func (app *application) logHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next = hlog.RequestIDHandler("req_id", "Request-Id")(next)

		err := r.ParseForm()
		if err != nil {
			app.clientError(w, http.StatusBadRequest)
			return
		}

		if app.Debug {
			// If any of those are empty, they won't get logged
			app.enrichDebugLogContext(r, "get_params", r.URL.Query().Encode())
			app.enrichDebugLogContext(r, "post_params", r.PostForm.Encode())
		}

		if app.LogRequests || app.Debug {
			next = hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
				hlog.FromRequest(r).Info().
					Str("method", r.Method).
					Stringer("url", r.URL).
					Int("status", status).
					Int("size", size).
					// TODO: leave duration like that?
					Str("duration", duration.String()).
					Msg("")
			})(next)
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
		if app.SafeMode && app.isUnsafePath(r.URL.Path) {
			// TODO: change logging level?
			hlog.FromRequest(r).Error().Caller().
				Msgf("Blocked a request to %s", r.URL.Path)
			app.clientError(w, http.StatusForbidden)
			return
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
			app.clientErrorMessage(w, http.StatusUnauthorized, err)
			return
		}

		ctx := r.Context()
		accessToken, err := app.verifier.Verify(ctx, rawAccessToken)
		if err != nil {
			// Better to log to see token verification errors
			hlog.FromRequest(r).Error().Caller().
				Err(err).Msgf("")
			app.clientErrorMessage(w, http.StatusUnauthorized, err)
			return
		}

		// Extract custom claims
		var claims struct {
			Roles []string `json:"roles"`
			Email string   `json:"email"`
			// Username string   `json:"preferred_username"`
		}
		if err := accessToken.Claims(&claims); err != nil {
			// Claims not set, bad token
			hlog.FromRequest(r).Error().Caller().
				Err(err).Msgf("")
			app.clientErrorMessage(w, http.StatusUnauthorized, err)
			return
		}

		app.enrichLogContext(r, "email", claims.Email)

		userRoles, err := app.getUserRoles(claims.Roles)
		if err != nil {
			hlog.FromRequest(r).Error().Caller().
				Err(err).Msgf("")
			app.clientErrorMessage(w, http.StatusUnauthorized, err)
			return
		}

		lf, err := app.getLF(userRoles)
		if err != nil {
			hlog.FromRequest(r).Error().Caller().
				Err(err).Msgf("")
			app.clientErrorMessage(w, http.StatusUnauthorized, err)
			return
		}
		// TODO: change name?
		app.enrichDebugLogContext(r, "label_filter", string(lf.AppendString(nil)))

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

		if app.isNotAPIRequest(r.URL.Path) {
			hlog.FromRequest(r).Debug().Caller().
				Msg("Not an API request, request is not modified")
			next.ServeHTTP(w, r)
			return
		}

		hasFullaccess, ok := r.Context().Value(contextKeyHasFullaccess).(bool)
		if ok && hasFullaccess {
			hlog.FromRequest(r).Debug().Caller().
				Msg("User has full access, request is not modified")
			next.ServeHTTP(w, r)
			return
		}

		lf, ok := r.Context().Value(contextKeyLabelFilter).(metricsql.LabelFilter)
		if !ok {
			app.serverError(w, r, fmt.Errorf("LF is not set in the context"))
			return
		}

		err := r.ParseForm()
		if err != nil {
			app.clientError(w, http.StatusBadRequest)
			return
		}

		// TODO: do only if it matches GET? Or it's safer to leave it as is?
		// Adjust GET params
		getParams := r.URL.Query()
		newGetParams, err := app.prepareQueryParams(&getParams, lf)
		if err != nil {
			// TODO: remove log?
			hlog.FromRequest(r).Error().Caller().
				Err(err).Msgf("")
			app.clientError(w, http.StatusBadRequest)
			return
		}
		r.URL.RawQuery = newGetParams
		// TODO: Optimize logging?
		app.enrichDebugLogContext(r, "new_get_params", newGetParams)

		// Adjust POST params
		// Partially inspired by https://github.com/bitsbeats/prometheus-acls/blob/master/internal/labeler/middleware.go
		if r.Method == http.MethodPost {
			newPostParams, err := app.prepareQueryParams(&r.PostForm, lf)
			if err != nil {
				hlog.FromRequest(r).Error().Caller().
					Err(err).Msgf("")
				app.clientError(w, http.StatusBadRequest)
				return
			}
			newBody := strings.NewReader(newPostParams)
			r.ContentLength = newBody.Size()
			r.Body = io.NopCloser(newBody)
			// TODO: somehow deal with not adjusted requests
			// TODO: add a field that would say that request stayed the same?
			// TODO: log these fields optionally? (add condition for debug or a custom parameter)
			app.enrichDebugLogContext(r, "new_post_params", newPostParams)
		}

		next.ServeHTTP(w, r)
	})
}
