package lfgw

import (
	"flag"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
)

func Test_newApplication(t *testing.T) {
	// All these tests make sure only one boolean gets changed to true. Otherwise, there's always a risk that value for one field overrides another one.
	tests := []struct {
		name string
		want application
	}{
		{
			name: "debug",
			want: application{Debug: true},
		},
		{
			name: "log-no-color",
			want: application{LogNoColor: true},
		},
		{
			name: "log-requests",
			want: application{LogRequests: true},
		},
		{
			name: "optimize-expressions",
			want: application{OptimizeExpressions: true},
		},
		{
			name: "enable-deduplication",
			want: application{EnableDeduplication: true},
		},
		{
			name: "safe-mode",
			want: application{SafeMode: true},
		},
		{
			name: "set-proxy-headers",
			want: application{SetProxyHeaders: true},
		},
		{
			name: "set-gomax-procs",
			want: application{SetGomaxProcs: true},
		},
		{
			name: "assumed-roles",
			want: application{AssumedRolesEnabled: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := flag.NewFlagSet("test", 0)
			set.Bool(tt.name, true, "doc")
			c := cli.NewContext(nil, set, nil)

			// Needed since Parse is called in the function
			tt.want.UpstreamURL = &url.URL{}

			got, err := newApplication(c)
			assert.Nil(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("Full application struct", func(t *testing.T) {
		upstreamURL := "http://localhost"
		oidcRealmURL := "http://localhost2"
		oidcClientID := "grafana"
		aclPath := "ACL.yaml"
		assumedRoles := true
		enableDeduplication := true
		optimizeExpression := true
		safeMode := true
		setProxyHeaders := true
		setGomaxProcs := true
		debug := true
		logFormat := "json"
		logNoColor := true
		logRequests := true
		port := 9999
		readTimeout := 6 * time.Second
		writeTimeout := 7 * time.Second
		gracefulShutdownTimeout := 8 * time.Second

		set := flag.NewFlagSet("test", 0)
		set.String("upstream-url", upstreamURL, "doc")
		set.String("oidc-realm-url", oidcRealmURL, "doc")
		set.String("oidc-client-id", oidcClientID, "doc")
		set.String("acl-path", aclPath, "doc")
		set.Bool("assumed-roles", assumedRoles, "doc")
		set.Bool("enable-deduplication", enableDeduplication, "doc")
		set.Bool("optimize-expressions", optimizeExpression, "doc")
		set.Bool("safe-mode", safeMode, "doc")
		set.Bool("set-proxy-headers", setProxyHeaders, "doc")
		set.Bool("set-gomax-procs", setGomaxProcs, "doc")
		set.Bool("debug", debug, "doc")
		set.String("log-format", logFormat, "doc")
		set.Bool("log-no-color", logNoColor, "doc")
		set.Bool("log-requests", logRequests, "doc")
		set.Int("port", port, "doc")
		set.Duration("read-timeout", readTimeout, "doc")
		set.Duration("write-timeout", writeTimeout, "doc")
		set.Duration("graceful-shutdown-timeout", gracefulShutdownTimeout, "doc")
		c := cli.NewContext(nil, set, nil)

		appUpstreamURL, err := url.Parse(upstreamURL)
		assert.Nil(t, err)

		want := application{
			UpstreamURL:             appUpstreamURL,
			OIDCRealmURL:            oidcRealmURL,
			OIDCClientID:            oidcClientID,
			ACLPath:                 aclPath,
			AssumedRolesEnabled:     assumedRoles,
			OptimizeExpressions:     optimizeExpression,
			EnableDeduplication:     enableDeduplication,
			SafeMode:                safeMode,
			SetProxyHeaders:         setProxyHeaders,
			SetGomaxProcs:           setGomaxProcs,
			Debug:                   debug,
			LogFormat:               logFormat,
			LogNoColor:              logNoColor,
			LogRequests:             logRequests,
			Port:                    port,
			ReadTimeout:             readTimeout,
			WriteTimeout:            writeTimeout,
			GracefulShutdownTimeout: gracefulShutdownTimeout,
		}

		got, err := newApplication(c)
		assert.Nil(t, err)

		assert.Equal(t, want, got)
	})
}
