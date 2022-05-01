package lfgw

import (
	"context"
	"fmt"
	"log"
	"net/http/httputil"
	"net/url"
	"runtime"
	"time"

	oidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
	"github.com/weisdd/lfgw/internal/querymodifier"
	"go.uber.org/automaxprocs/maxprocs"
)

// Define an application struct to hold the application-wide dependencies for the
// web application.
type application struct {
	UpstreamURL             *url.URL
	OIDCRealmURL            string
	OIDCClientID            string
	ACLPath                 string
	AssumedRolesEnabled     bool
	EnableDeduplication     bool
	OptimizeExpressions     bool
	SafeMode                bool
	SetProxyHeaders         bool
	SetGomaxProcs           bool
	Debug                   bool
	LogFormat               string
	LogNoColor              bool
	LogRequests             bool
	Port                    int
	ReadTimeout             time.Duration
	WriteTimeout            time.Duration
	GracefulShutdownTimeout time.Duration
	errorLog                *log.Logger
	ACLs                    querymodifier.ACLs
	proxy                   *httputil.ReverseProxy
	verifier                *oidc.IDTokenVerifier
	logger                  *zerolog.Logger
}

// Run is used as an entrypoint for cli
func Run(c *cli.Context) error {
	app, err := newApplication(c)
	if err != nil {
		return err
	}

	app.Run()

	return nil
}

// newApplication returns application struct built from *cli.Context
func newApplication(c *cli.Context) (application, error) {
	upstreamURL, err := url.Parse(c.String("upstream-url"))
	if err != nil {
		return application{}, fmt.Errorf("failed to parse upstream-url: %s", err)
	}

	app := application{
		UpstreamURL:             upstreamURL,
		OIDCRealmURL:            c.String("oidc-realm-url"),
		OIDCClientID:            c.String("oidc-client-id"),
		ACLPath:                 c.String("acl-path"),
		AssumedRolesEnabled:     c.Bool("assumed-roles"),
		EnableDeduplication:     c.Bool("enable-deduplication"),
		OptimizeExpressions:     c.Bool("optimize-expressions"),
		SafeMode:                c.Bool("safe-mode"),
		SetProxyHeaders:         c.Bool("set-proxy-headers"),
		SetGomaxProcs:           c.Bool("set-gomax-procs"),
		Debug:                   c.Bool("debug"),
		LogFormat:               c.String("log-format"),
		LogNoColor:              c.Bool("log-no-color"),
		LogRequests:             c.Bool("log-requests"),
		Port:                    c.Int("port"),
		ReadTimeout:             c.Duration("read-timeout"),
		WriteTimeout:            c.Duration("write-timeout"),
		GracefulShutdownTimeout: c.Duration("graceful-shutdown-timeout"),
	}

	return app, nil
}

// Run starts lfgw (main-like function)
func (app *application) Run() {
	app.configureLogging()
	app.configureACLs()
	app.configureOIDCVerifier()

	// TODO: expose undo and move to another function?
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

	err := app.serve()
	if err != nil {
		app.logger.Fatal().Caller().
			Err(err).Msg("")
	}
}

// configureACLs logs assumed roles mode, verifies current ACLs settings (assumed roles, aclpath), loads the ACLs from a file and logs roles if needed
func (app *application) configureACLs() {
	// Just to make sure our logging calls are always safe
	if app.logger == nil {
		app.configureLogging()
	}

	if app.AssumedRolesEnabled {
		app.logger.Info().Caller().
			Msg("Assumed roles mode is on")
	} else {
		app.logger.Info().Caller().
			Msg("Assumed roles mode is off")
	}

	if app.ACLPath == "" {
		// NOTE: the condition should never happen as it's filtered out by "Before" functionality of cli, though left just in case
		if !app.AssumedRolesEnabled {
			app.logger.Fatal().Caller().
				Msgf("The app cannot run without at least one source of configuration (Non-empty ACL_PATH and/or ASSUMED_ROLES set to true)")
		}

		app.logger.Info().Caller().
			Msgf("ACL_PATH is empty, thus predefined roles will not be loaded")

		return
	}

	var err error

	app.ACLs, err = querymodifier.NewACLsFromFile(app.ACLPath)
	if err != nil {
		app.logger.Fatal().Caller().
			Err(err).Msgf("Failed to load ACL")
	}

	for role, acl := range app.ACLs {
		app.logger.Info().Caller().
			Msgf("Loaded role definition for %s: %q (converted to %s)", role, acl.RawACL, acl.LabelFilter.AppendString(nil))
	}
}

// configureOIDCVerifier sets up OIDC token verifier by using app.OIDCRealmURL and app.OIDCClientID
func (app *application) configureOIDCVerifier() {
	// Just to make sure our logging calls are always safe
	if app.logger == nil {
		app.configureLogging()
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
}
