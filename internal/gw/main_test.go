package gw

import (
	"flag"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
)

func Test_newApplication(t *testing.T) {
	// TODO: test url error
	// TODO: test boolean independently to make sure they don't override each other
	debug := true
	logFormat := "json"
	logNoColor := true
	logRequests := true
	upstreamURL := "http://localhost"
	optimizeExpression := true
	enableDeduplication := true
	safeMode := true
	setProxyHeaders := true
	setGomaxProcs := true
	aclPath := "ACL.yaml"
	assumedRoles := true
	oidcRealmURL := "http://localhost2"
	oidcClientID := "grafana"
	port := 9999
	readTimeout := 6 * time.Second
	writeTimeout := 7 * time.Second
	gracefulShutdownTimeout := 8 * time.Second

	set := flag.NewFlagSet("test", 0)
	set.Bool("debug", debug, "doc")
	set.String("log-format", logFormat, "doc")
	set.Bool("log-no-color", logNoColor, "doc")
	set.Bool("log-requests", logRequests, "doc")
	set.String("upstream-url", upstreamURL, "doc")
	set.Bool("optimize-expressions", optimizeExpression, "doc")
	set.Bool("enable-deduplication", enableDeduplication, "doc")
	set.Bool("safe-mode", safeMode, "doc")
	set.Bool("set-proxy-headers", setProxyHeaders, "doc")
	set.Bool("set-gomax-procs", setGomaxProcs, "doc")
	set.String("acl-path", aclPath, "doc")
	set.Bool("assumed-roles", assumedRoles, "doc")
	set.String("oidc-realm-url", oidcRealmURL, "doc")
	set.String("oidc-client-id", oidcClientID, "doc")
	set.Int("port", port, "doc")
	set.Duration("read-timeout", readTimeout, "doc")
	set.Duration("write-timeout", writeTimeout, "doc")
	set.Duration("graceful-shutdown-timeout", gracefulShutdownTimeout, "doc")
	c := cli.NewContext(nil, set, nil)

	appUpstreamURL, err := url.Parse(upstreamURL)
	assert.Nil(t, err)

	want := application{
		Debug:                   debug,
		LogFormat:               logFormat,
		LogNoColor:              logNoColor,
		LogRequests:             logRequests,
		UpstreamURL:             appUpstreamURL,
		OptimizeExpressions:     optimizeExpression,
		EnableDeduplication:     enableDeduplication,
		SafeMode:                safeMode,
		SetProxyHeaders:         setProxyHeaders,
		SetGomaxProcs:           setGomaxProcs,
		ACLPath:                 aclPath,
		AssumedRolesEnabled:     assumedRoles,
		OIDCRealmURL:            oidcRealmURL,
		OIDCClientID:            oidcClientID,
		Port:                    port,
		ReadTimeout:             readTimeout,
		WriteTimeout:            writeTimeout,
		GracefulShutdownTimeout: gracefulShutdownTimeout,
	}

	got, err := newApplication(c)
	assert.Nil(t, err)

	assert.Equal(t, want, got)
}
