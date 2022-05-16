package lfgw

import "errors"

var (
	errNoToken                = errors.New("no bearer token found")
	errNoTokenGrafana         = errors.New("no bearer token found, possible causes: grafana data source is not configured with Forward Oauth Identity option; grafana user sessions are not tuned to live shorter than IDP sessions; malicious requests")
	errUpstreamNotInitialized = errors.New("UpstreamURL is not initialized")
	errVerifierNotInitialized = errors.New("OIDC verifier is not initialized")
	errACLNotSetInContext     = errors.New("ACL is not set in the context")
)
