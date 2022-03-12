package main

import (
	"context"
	"log"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/caarlos0/env/v6"
	oidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

// Define an application struct to hold the application-wide dependencies for the
// web application.
type application struct {
	errorLog                *log.Logger
	logger                  *zerolog.Logger
	ACLMap                  ACLMap
	proxy                   *httputil.ReverseProxy
	verifier                *oidc.IDTokenVerifier
	Debug                   bool          `env:"DEBUG" envDefault:"false"`
	UpstreamURL             *url.URL      `env:"UPSTREAM_URL,required"`
	OptimizeExpressions     bool          `env:"OPTIMIZE_EXPRESSIONS" envDefault:"true"`
	SafeMode                bool          `env:"SAFE_MODE" envDefault:"true"`
	SetProxyHeaders         bool          `env:"SET_PROXY_HEADERS" envDefefault:"false"`
	ACLPath                 string        `env:"ACL_PATH" envDefault:"./acl.yaml"`
	OIDCRealmURL            string        `env:"OIDC_REALM_URL,required"`
	OIDCClientID            string        `env:"OIDC_CLIENT_ID,required"`
	Port                    int           `env:"PORT" envDefault:"8080"`
	ReadTimeout             time.Duration `env:"READ_TIMEOUT" envDefault:"10s"`
	WriteTimeout            time.Duration `env:"WRITE_TIMEOUT" envDefault:"10s"`
	GracefulShutdownTimeout time.Duration `env:"GRACEFUL_SHUTDOWN_TIMEOUT" envDefault:"20s"`
}

type contextKey string

const contextKeyHasFullaccess = contextKey("hasFullaccess")
const contextKeyLabelFilter = contextKey("labelFilter")

// TODO: move to another file
type stdErrorWrapper struct {
	logger *zerolog.Logger
}

// TODO: new?

func (s stdErrorWrapper) Write(p []byte) (n int, err error) {
	msg := string(p)
	msg = strings.TrimSpace(msg)

	var errorMsg string
	var caller string
	// TODO: move logic to callerHook?
	for i := range msg {
		if msg[i] == ' ' {
			// Skip ":"
			caller = msg[:i-1]
			// length should always be fine as we trim spaces, thus there can't be a trailing space
			errorMsg = msg[i+1:]
			break
		}
	}

	s.logger.Error().
		Str("caller", caller).
		Str("error", errorMsg).
		Msgf("")

	return len(p), nil
}

func main() {
	logWrapper := stdErrorWrapper{logger: &zlog.Logger}
	errorLog := log.New(logWrapper, "", log.Lshortfile)

	// TODO: create a new logger?
	zerolog.CallerMarshalFunc = func(file string, line int) string {
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

	app := &application{
		logger:   &zlog.Logger,
		errorLog: errorLog,
	}

	err := env.Parse(app)
	if err != nil {
		app.logger.Fatal().Caller().Err(err).Msgf("")
	}

	if app.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	app.ACLMap, err = app.loadACL()
	if err != nil {
		app.logger.Fatal().Caller().Err(err).Msgf("")
	}

	app.logger.Info().Caller().
		Msgf("Connecting to OIDC backend (%q)", app.OIDCRealmURL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	provider, err := oidc.NewProvider(ctx, app.OIDCRealmURL)
	if err != nil {
		app.logger.Fatal().Caller().Err(err).Msgf("")
	}

	oidcConfig := &oidc.Config{
		ClientID: app.OIDCClientID,
	}
	app.verifier = provider.Verifier(oidcConfig)

	app.proxy = httputil.NewSingleHostReverseProxy(app.UpstreamURL)
	// TODO: somehow pass more context to ErrorLog
	app.proxy.ErrorLog = app.errorLog
	app.proxy.FlushInterval = time.Millisecond * 200

	err = app.serve()
	if err != nil {
		app.logger.Fatal().Caller().Err(err).Msgf("")
	}
}
