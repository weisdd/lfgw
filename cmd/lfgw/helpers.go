package main

import (
	"fmt"
	"net/http"
	"strconv"
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

// lshortfile implements Lshortfile equivalent for zerolog's CallerMarshalFunc
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
