package lfgw

import (
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/hlog"
)

// routes returns a router with all paths.
func (app *application) routes() *mux.Router {
	r := mux.NewRouter()
	r.Use(app.nonProxiedEndpointsMiddleware)
	r.Use(hlog.NewHandler(*app.logger))
	r.Use(app.logMiddleware)
	r.Use(app.oidcModeMiddleware)
	// Better to keep it here to see user email in logs (for unsafe paths)
	r.Use(app.safeModeMiddleware)
	r.Use(app.proxyHeadersMiddleware)
	r.Use(app.rewriteRequestMiddleware)
	r.PathPrefix("/").Handler(app.proxy)
	return r
}
