package main

import (
	"context"
	"log"
	"net/http/httputil"
	"net/url"
	"os"
	"runtime"
	"time"

	"github.com/caarlos0/env/v6"
	oidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"go.uber.org/automaxprocs/maxprocs"
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
	LogFormat               string        `env:"LOG_FORMAT" envDefault:"pretty"`
	LogRequests             bool          `env:"LOG_REQUESTS" envDefault:"false"`
	LogNoColor              bool          `env:"LOG_NO_COLOR" envDefault:"false"`
	UpstreamURL             *url.URL      `env:"UPSTREAM_URL,required"`
	OptimizeExpressions     bool          `env:"OPTIMIZE_EXPRESSIONS" envDefault:"true"`
	EnableDeduplication     bool          `env:"ENABLE_DEDUPLICATION" envDefault:"true"`
	SafeMode                bool          `env:"SAFE_MODE" envDefault:"true"`
	SetProxyHeaders         bool          `env:"SET_PROXY_HEADERS" envDefault:"false"`
	SetGomaxProcs           bool          `env:"SET_GOMAXPROCS" envDefault:"true"`
	ACLPath                 string        `env:"ACL_PATH" envDefault:"./acl.yaml"`
	AssumedRoles            bool          `env:"ASSUMED_ROLES" envDefault:"false"`
	OIDCRealmURL            string        `env:"OIDC_REALM_URL,required"`
	OIDCClientID            string        `env:"OIDC_CLIENT_ID,required"`
	Port                    int           `env:"PORT" envDefault:"8080"`
	ReadTimeout             time.Duration `env:"READ_TIMEOUT" envDefault:"10s"`
	WriteTimeout            time.Duration `env:"WRITE_TIMEOUT" envDefault:"10s"`
	GracefulShutdownTimeout time.Duration `env:"GRACEFUL_SHUTDOWN_TIMEOUT" envDefault:"20s"`
}

type contextKey string

const contextKeyACL = contextKey("acl")

func main() {
	zlog.Logger = zlog.Output(os.Stdout)

	logWrapper := stdErrorLogWrapper{logger: &zlog.Logger}
	// NOTE: don't delete log.Lshortfile
	errorLog := log.New(logWrapper, "", log.Lshortfile)

	app := &application{
		logger:   &zlog.Logger,
		errorLog: errorLog,
	}

	zerolog.CallerMarshalFunc = app.lshortfile
	zerolog.DurationFieldUnit = time.Second

	err := env.Parse(app)
	if err != nil {
		app.logger.Fatal().Caller().
			Err(err).Msg("")
	}

	// TODO: think of something better?
	if app.LogFormat == "pretty" {
		zlog.Logger = zlog.Output(zerolog.ConsoleWriter{Out: os.Stdout, NoColor: app.LogNoColor})
	}

	if app.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if app.SetGomaxProcs {
		undo, err := maxprocs.Set()
		defer undo()
		if err != nil {
			app.logger.Error().Caller().
				Msgf("failed to set GOMAXPROCS: %v", err)
		}
	}
	app.logger.Info().Caller().
		Msgf("Runtime settings: GOMAXPROCS = %d", runtime.GOMAXPROCS(0))

	if app.AssumedRoles {
		app.logger.Info().Caller().
			Msg("Assumed roles mode is on")
	} else {
		app.logger.Info().Caller().
			Msg("Assumed roles mode is off")
	}

	if app.ACLPath != "" {
		app.ACLMap, err = app.loadACL()
		if err != nil {
			app.logger.Fatal().Caller().
				Err(err).Msgf("Failed to load ACL")
		}

		for role, acl := range app.ACLMap {
			app.logger.Info().Caller().
				Msgf("Loaded role definition for %s: %q (converted to %s)", role, acl.RawACL, acl.LabelFilter.AppendString(nil))
		}
	} else {
		if !app.AssumedRoles {
			app.logger.Fatal().Caller().
				Msgf("The app cannot run without at least one source of configuration (Non-empty ACL_PATH and/or ASSUMED_ROLES set to true)")
		}

		app.logger.Info().Caller().
			Msgf("ACL_PATH is empty, thus predefined roles are not loaded")
	}

	app.logger.Info().Caller().
		Msgf("Connecting to OIDC backend (%q)", app.OIDCRealmURL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	provider, err := oidc.NewProvider(ctx, app.OIDCRealmURL)
	if err != nil {
		app.logger.Fatal().Caller().
			Err(err).Msg("")
	}

	oidcConfig := &oidc.Config{
		ClientID: app.OIDCClientID,
	}
	app.verifier = provider.Verifier(oidcConfig)

	app.proxy = httputil.NewSingleHostReverseProxy(app.UpstreamURL)
	// TODO: somehow pass more context to ErrorLog (unsafe?)
	app.proxy.ErrorLog = app.errorLog
	app.proxy.FlushInterval = time.Millisecond * 200

	err = app.serve()
	if err != nil {
		app.logger.Fatal().Caller().
			Err(err).Msg("")
	}
}
