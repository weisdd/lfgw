package gw

import (
	"context"
	"fmt"
	"log"
	"net/http/httputil"
	"net/url"
	"os"
	"runtime"
	"time"

	oidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
	"github.com/weisdd/lfgw/internal/querymodifier"
	"go.uber.org/automaxprocs/maxprocs"
)

// Define an application struct to hold the application-wide dependencies for the
// web application.
type application struct {
	errorLog                *log.Logger
	logger                  *zerolog.Logger
	ACLs                    querymodifier.ACLs
	proxy                   *httputil.ReverseProxy
	verifier                *oidc.IDTokenVerifier
	Debug                   bool
	LogFormat               string
	LogNoColor              bool
	LogRequests             bool
	UpstreamURL             *url.URL
	OptimizeExpressions     bool
	EnableDeduplication     bool
	SafeMode                bool
	SetProxyHeaders         bool
	SetGomaxProcs           bool
	ACLPath                 string
	AssumedRolesEnabled     bool
	OIDCRealmURL            string
	OIDCClientID            string
	Port                    int
	ReadTimeout             time.Duration
	WriteTimeout            time.Duration
	GracefulShutdownTimeout time.Duration
}

type contextKey string

const contextKeyACL = contextKey("acl")

func Run(c *cli.Context) error {
	// TODO: move conversion further?
	// TODO: check that all default values were correctly propagated
	// TODO: check the same
	// TODO: tests for each option to make sure values change
	upstreamURL, err := url.Parse(c.String("upstream-url"))
	if err != nil {
		return err
	} else if upstreamURL.Host == "" {
		return fmt.Errorf("upstream-url cannot be empty")
	}

	app := application{
		Debug:                   c.Bool("debug"),
		LogFormat:               c.String("log-format"),
		LogNoColor:              c.Bool("log-no-color"),
		LogRequests:             c.Bool("log-requests"),
		UpstreamURL:             upstreamURL,
		OptimizeExpressions:     c.Bool("optimize-expressions"),
		EnableDeduplication:     c.Bool("enable-deduplication"),
		SafeMode:                c.Bool("safe-mode"),
		SetProxyHeaders:         c.Bool("set-proxy-headers"),
		SetGomaxProcs:           c.Bool("set-gomax-procs"),
		ACLPath:                 c.String("acl-path"),
		AssumedRolesEnabled:     c.Bool("assumed-roles"),
		OIDCRealmURL:            c.String("oidc-realm-url"),
		OIDCClientID:            c.String("oidc-client-id"),
		Port:                    c.Int("port"),
		ReadTimeout:             c.Duration("read-timeout"),
		WriteTimeout:            c.Duration("write-timeout"),
		GracefulShutdownTimeout: c.Duration("graceful-shutdown-timeout"),
	}

	run(app)

	return nil
}

func run(app application) {
	zlog.Logger = zlog.Output(os.Stdout)
	app.logger = &zlog.Logger
	logWrapper := stdErrorLogWrapper{logger: app.logger}
	// NOTE: don't delete log.Lshortfile
	app.errorLog = log.New(logWrapper, "", log.Lshortfile)

	zerolog.CallerMarshalFunc = app.lshortfile
	zerolog.DurationFieldUnit = time.Second

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

	if app.AssumedRolesEnabled {
		app.logger.Info().Caller().
			Msg("Assumed roles mode is on")
	} else {
		app.logger.Info().Caller().
			Msg("Assumed roles mode is off")
	}

	var err error

	if app.ACLPath != "" {
		app.ACLs, err = querymodifier.NewACLsFromFile(app.ACLPath)
		if err != nil {
			app.logger.Fatal().Caller().
				Err(err).Msgf("Failed to load ACL")
		}

		for role, acl := range app.ACLs {
			app.logger.Info().Caller().
				Msgf("Loaded role definition for %s: %q (converted to %s)", role, acl.RawACL, acl.LabelFilter.AppendString(nil))
		}
	} else {
		if !app.AssumedRolesEnabled {
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
