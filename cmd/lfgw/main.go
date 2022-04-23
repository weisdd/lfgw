package main

import (
	"time"

	"github.com/urfave/cli/v2"
	"github.com/weisdd/lfgw/internal/gw"

	"fmt"
	"os"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	app := &cli.App{
		Name: "lfgw",
		// TODO: pass value through Dockerfile
		Version: fmt.Sprintf("%s (commit: %s)", version, commit),
		// TODO: can't find where it's printed
		Compiled: time.Now(),
		Authors: []*cli.Author{
			{
				Name: "weisdd",
			},
		},
		Copyright: "© 2021-2022 weisdd",
		HelpName:  "lfgw",
		Usage:     "A reverse proxy aimed at PromQL / MetricsQL metrics filtering based on OIDC roles",
		UsageText: "lfgw [flags]",
		// UseShortOptionHandling: true,
		// EnableBashCompletion:   true,
		HideHelpCommand: true,
		Action:          gw.Run,
		Before: func(c *cli.Context) error {
			nonEmptyStrings := []string{"upstream-url", "oidc-realm-url", "oidc-client-id"}

			for _, key := range nonEmptyStrings {
				if c.String(key) == "" {
					return fmt.Errorf("%s cannot be empty", key)
				}
			}

			if c.String("acl-path") == "" && !c.Bool("assumed-roles") {
				return fmt.Errorf("the app cannot run without at least one configuration source: defined acl-path or assumed-roles set to true")
			}

			return nil
		},
		// TODO: reorder
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:     "debug",
				Usage:    "whether to print out debug log messages",
				EnvVars:  []string{"DEBUG"},
				Value:    false,
				Required: false,
			},
			&cli.StringFlag{
				Name:     "log-format",
				Usage:    "log format: pretty, json",
				EnvVars:  []string{"LOG_FORMAT"},
				Value:    "pretty",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "log-no-color",
				Usage:    "whether to disable colors for pretty format",
				EnvVars:  []string{"LOG_NO_COLOR"},
				Value:    false,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "log-requests",
				Usage:    "whether to log HTTP requests",
				EnvVars:  []string{"LOG_REQUESTS"},
				Value:    false,
				Required: false,
			},
			&cli.StringFlag{
				Name:     "upstream-url",
				Usage:    "Prometheus URL, e.g. http://prometheus.microk8s.localhost",
				EnvVars:  []string{"UPSTREAM_URL"},
				Required: true,
			},
			&cli.BoolFlag{
				Name:     "optimize-expressions",
				Usage:    "whether to automatically optimize expressions for non-full access requests",
				EnvVars:  []string{"OPTIMIZE_EXPRESSIONS"},
				Value:    true,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "enable-deduplication",
				Usage:    "whether to enable deduplication, which leaves some of the requests unmodified if they match the target policy",
				EnvVars:  []string{"ENABLE_DEDUPLICATION"},
				Value:    true,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "safe-mode",
				Usage:    "whether to block requests to sensitive endpoints (tsdb admin, insert)",
				EnvVars:  []string{"SAFE_MODE"},
				Value:    true,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "set-proxy-headers",
				Usage:    "whether to set proxy headers (X-Forwarded-For, X-Forwarded-Proto, X-Forwarded-Host)",
				EnvVars:  []string{"SET_PROXY_HEADERS"},
				Value:    false,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "set-gomax-procs",
				Usage:    "automatically set GOMAXPROCS to match Linux container CPU quota",
				EnvVars:  []string{"SET_GOMAXPROCS"},
				Value:    true,
				Required: false,
			},
			&cli.StringFlag{
				Name:     "acl-path",
				Usage:    "path to a file with ACL definitions (OIDC role to namespace bindings), skipped if empty",
				EnvVars:  []string{"ACL_PATH"},
				Value:    "./acl.yaml",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "assumed-roles",
				Usage:    "whether to treat unknown OIDC-role names as acl definitions",
				EnvVars:  []string{"ASSUMED_ROLES"},
				Value:    false,
				Required: false,
			},
			&cli.StringFlag{
				Name:     "oidc-realm-url",
				Usage:    "OIDC Realm URL, e.g. `https://auth.microk8s.localhost/auth/realms/cicd",
				EnvVars:  []string{"OIDC_REALM_URL"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "oidc-client-id",
				Usage:    "OIDC Client ID (used for token audience validation)",
				EnvVars:  []string{"OIDC_CLIENT_ID"},
				Required: true,
			},
			&cli.IntFlag{
				Name:     "port",
				Usage:    "port the web server will listen on",
				EnvVars:  []string{"PORT"},
				Value:    8080,
				Required: false,
			},
			// TODO: check duration is correct by pressing Ctrl+C
			&cli.DurationFlag{
				Name:     "read-timeout",
				Usage:    "the maximum time the from when the connection is accepted to when the request body is fully read",
				EnvVars:  []string{"READ_TIMEOUT"},
				Value:    10 * time.Second,
				Required: false,
			},
			&cli.DurationFlag{
				Name:     "write-timeout",
				Usage:    "the maximum time from the end of the request header read to the end of the response write",
				EnvVars:  []string{"WRITE_TIMEOUT"},
				Value:    10 * time.Second,
				Required: false,
			},
			&cli.DurationFlag{
				Name:     "graceful-shutdown-timeout",
				Usage:    "the maximum amount of time to wait for all connections to be closed",
				EnvVars:  []string{"GRACEFUL_SHUTDOWN_TIMEOUT"},
				Value:    20 * time.Second,
				Required: false,
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Printf("%+v: %+v", os.Args[0], err)
	}
}
