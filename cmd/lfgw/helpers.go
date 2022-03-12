package main

import (
	"fmt"
	"net/http"
	"strings"
)

// serverError sends a generic 500 Internal Server Error response to the user.
func (app *application) serverError(w http.ResponseWriter, err error) {
	app.logger.Error().Caller().Err(err).Msgf("")
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
