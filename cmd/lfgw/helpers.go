package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
)

// serverError sends a generic 500 Internal Server Error response to the user.
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	hlog.FromRequest(r).Error().Caller(1).
		Err(err).Msg("")
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// clientError sends responses like 400 "Bad Request" to the user.
func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

// clientErrorMessage sends responses like 400 "Bad Request" to the user with an additional message.
func (app *application) clientErrorMessage(w http.ResponseWriter, status int, err error) {
	http.Error(w, http.StatusText(status), status)
	fmt.Fprintf(w, "%s", err)
}

// getRawAccessToken returns a raw access token
func (app *application) getRawAccessToken(r *http.Request) (string, error) {
	headers := []string{"X-Forwarded-Access-Token", "X-Auth-Request-Access-Token", "Authorization"}

	for _, h := range headers {
		t := r.Header.Get(h)

		if h == "Authorization" {
			t = strings.TrimPrefix(t, "Bearer ")
		}

		if t != "" {
			return t, nil
		}
	}

	return "", fmt.Errorf("no bearer token found")
}

// lshortfile implements Lshortfile equivalent for zerolog's CallerMarshalFunc.
func (app *application) lshortfile(file string, line int) string {
	// Copied from the standard library: https://cs.opensource.google/go/go/+/refs/tags/go1.17.8:src/log/log.go;drc=926994fd7cf65b2703552686965fb05569699897;l=134
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}
	file = short
	return file + ":" + strconv.Itoa(line)
}

// enrichLogContext adds a custom field and a value to zerolog context.
func (app *application) enrichLogContext(r *http.Request, field string, value string) {
	if field != "" && value != "" {
		log := zerolog.Ctx(r.Context())
		log.UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Str(field, value)
		})
	}
}

// enrichDebugLogContext adds a custom field and a value to zerolog context if logging level is set to Debug.
func (app *application) enrichDebugLogContext(r *http.Request, field string, value string) {
	if app.Debug {
		if field != "" && value != "" {
			log := zerolog.Ctx(r.Context())
			log.UpdateContext(func(c zerolog.Context) zerolog.Context {
				return c.Str(field, value)
			})
		}
	}
}

// isNotAPIRequest returns true if the requested path does not target API or federate endpoints.
func (app *application) isNotAPIRequest(path string) bool {
	return !strings.Contains(path, "/api/") && !strings.Contains(path, "/federate")
}

// isUnsafePath returns true if the requested path targets a potentially dangerous endpoint (admin or remote write).
func (app *application) isUnsafePath(path string) bool {
	// TODO: move to regexp?
	// TODO: more unsafe paths?
	return strings.Contains(path, "/admin/tsdb") || strings.Contains(path, "/api/v1/write")
}

// unescapedURLQuery returns unescaped query string
func (app *application) unescapedURLQuery(s string) string {
	// We should never hit an error as we encoded query string ourselves. The undelying library returns an empty string in case of an error, error handling is left only for clarity.
	decoded, err := url.QueryUnescape(s)
	if err != nil {
		return ""
	}

	return decoded
}
