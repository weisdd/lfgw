package main

import (
	"github.com/gorilla/mux"
)

// routes returns a router with all paths.
func (app *application) routes() *mux.Router {
	r := mux.NewRouter()
	r.Use(app.healthzMiddleware)
	r.Use(app.prohibitedMethodsMiddleware)
	r.Use(app.prohibitedPathsMiddleware)
	r.Use(app.proxyHeadersMiddleware)
	r.Use(app.oidcModeMiddleware)
	r.Use(app.rewriteRequestMiddleware)
	r.PathPrefix("/").Handler(app.proxy)
	return r
}
