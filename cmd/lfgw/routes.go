package main

import (
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/hlog"
)

// routes returns a router with all paths.
func (app *application) routes() *mux.Router {
	r := mux.NewRouter()
	r.Use(app.healthzMiddleware)
	// TODO: move to middleware accesslog?
	r.Use(hlog.NewHandler(*app.logger))
	r.Use(app.accessLogHandler)
	r.Use(app.prohibitedMethodsMiddleware)
	r.Use(app.proxyHeadersMiddleware)
	r.Use(app.oidcModeMiddleware)
	// Better to keep it here to see user email in logs
	r.Use(app.prohibitedPathsMiddleware)
	r.Use(app.rewriteRequestMiddleware)
	r.PathPrefix("/").Handler(app.proxy)
	return r
}
