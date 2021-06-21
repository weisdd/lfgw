package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/caarlos0/env/v6"
	oidc "github.com/coreos/go-oidc/v3/oidc"
)

// Define an application struct to hold the application-wide dependencies for the
// web application.
type application struct {
	errorLog        *log.Logger
	infoLog         *log.Logger
	debugLog        *log.Logger
	ACLMap          ACLMap
	proxy           *httputil.ReverseProxy
	verifier        *oidc.IDTokenVerifier
	Debug           bool          `env:"DEBUG" envDefault:"false"`
	LFGWMode        string        `env:"LFGW_MODE" envDefault:"oidc"` //TODO: only oidc supported at the moment
	UpstreamURL     *url.URL      `env:"UPSTREAM_URL,required"`
	SafeMode        bool          `env:"SAFE_MODE" envDefault:"true"`
	SetProxyHeaders bool          `env:"SET_PROXY_HEADERS" envDefefault:"false"`
	ACLPath         string        `env:"ACL_PATH" envDefault:"./acl.yaml"` //TODO: should be required only for oidc
	OIDCRealmURL    string        `env:"OIDC_REALM_URL,required"`          //TODO: should be required only for oidc
	OIDCClientID    string        `env:"OIDC_CLIENT_ID,required"`          //TODO: should be required only for oidc
	Port            int           `env:"PORT" envDefault:"8080"`
	ReadTimeout     time.Duration `env:"READ_TIMEOUT" envDefault:"10s"`
	WriteTimeout    time.Duration `env:"WRITE_TIMEOUT" envDefault:"10s"`
}

type contextKey string

const contextKeyHasFullaccess = contextKey("hasFullaccess")
const contextKeyLabelFilter = contextKey("labelFilter")

func main() {
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	debugLog := log.New(os.Stdout, "DEBUG\t", log.Ldate|log.Ltime|log.Lshortfile)

	app := &application{
		errorLog: errorLog,
		infoLog:  infoLog,
		debugLog: debugLog,
	}

	err := env.Parse(app)
	if err != nil {
		app.errorLog.Fatalf("%+v\n", err)
	}

	// TODO: remove when other modes are ported back
	if app.LFGWMode != "oidc" {
		app.errorLog.Fatal("Only oidc mode is currently supported")
	}

	if !app.Debug {
		app.debugLog.SetOutput(ioutil.Discard)
	}

	app.ACLMap, err = app.loadACL()
	if err != nil {
		app.errorLog.Fatal(err)
	}

	app.infoLog.Printf("Connecting to OIDC backend (%q)", app.OIDCRealmURL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	provider, err := oidc.NewProvider(ctx, app.OIDCRealmURL)
	if err != nil {
		app.errorLog.Fatal(err)
	}

	oidcConfig := &oidc.Config{
		ClientID: app.OIDCClientID,
	}
	app.verifier = provider.Verifier(oidcConfig)

	app.proxy = httputil.NewSingleHostReverseProxy(app.UpstreamURL)
	app.proxy.ErrorLog = app.errorLog
	app.proxy.FlushInterval = time.Millisecond * 200

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.Port),
		ErrorLog:     app.errorLog,
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  app.ReadTimeout,
		WriteTimeout: app.WriteTimeout,
	}

	app.infoLog.Printf("Starting server on %d", app.Port)
	err = srv.ListenAndServe()
	app.errorLog.Fatal(err)
}
