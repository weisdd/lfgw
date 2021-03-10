package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/metricsql"
)

// serverError sends a generic 500 Internal Server Error response to the user.
func (app *application) serverError(w http.ResponseWriter, err error) {
	app.errorLog.Println(err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// clientError sends responses like 400 "Bad Request" to the user.
func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

// clientError sends responses like 400 "Bad Request" to the user with an additional message.
func (app *application) clientErrorMessage(w http.ResponseWriter, status int, err error) {
	http.Error(w, http.StatusText(status), status)
	fmt.Fprintf(w, "%s", err)
}

// TODO: refactor/move somewhere else
// prepareRawQueryParams rewwrites GET "query" and "match" parameters to filter out metrics.
func (app *application) prepareRawQueryParams(r *http.Request, lf metricsql.LabelFilter) (string, error) {
	rawQueryParams := &url.Values{}

	err := r.ParseForm()
	if err != nil {
		return "", err
	}

	// TODO: refactor code
	for k, vv := range r.Form {
		switch k {
		case "query", "match[]":
			for _, v := range vv {
				{
					newVal, err := app.modifyMetricExpr(v, lf)
					if err != nil {
						return "", err
					}
					rawQueryParams.Add(k, newVal)
				}
			}
		default:
			for _, v := range vv {
				rawQueryParams.Add(k, v)
			}
		}
	}
	return rawQueryParams.Encode(), nil
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

	return "", fmt.Errorf("No bearer token found")
}
