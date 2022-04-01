package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/VictoriaMetrics/metricsql"
	"gopkg.in/yaml.v3"
)

// ACLMap stores a parsed YAML with role defitions
type ACLMap map[string]*ACL

// ACL stores a role definition
type ACL struct {
	Fullaccess  bool
	LabelFilter metricsql.LabelFilter
	RawACL      string
}

// toSlice converts namespace rules to string slices.
func (a *ACL) toSlice(str string) ([]string, error) {
	buffer := []string{}
	// yaml values should come already with trimmed leading and trailing spaces
	for _, s := range strings.Split(str, ",") {
		// in case there are empty elements in between
		s := strings.TrimSpace(s)

		// TODO: optionally disable it when things are loaded from a file?
		for _, ch := range s {
			if unicode.IsSpace(ch) {
				return nil, fmt.Errorf("line should not contain spaces within individual elements (%q)", str)
			}
		}

		if s != "" {
			buffer = append(buffer, s)
		}
	}

	if len(buffer) == 0 {
		return nil, fmt.Errorf("line has to contain at least one valid element (%q)", str)
	}

	return buffer, nil
}

// PrepareLF returns a label filter based on a rule definition (non-regexp for one namespace, regexp - for many)
func (a *ACL) PrepareLF(rawACL string) (metricsql.LabelFilter, error) {
	var lf = metricsql.LabelFilter{
		Label:      "namespace",
		IsNegative: false,
	}

	buffer, err := a.toSlice(rawACL)
	if err != nil {
		return metricsql.LabelFilter{}, err
	}

	// If .* is in the slice, then we can omit any other value
	for _, v := range buffer {
		// TODO: move to HasFullaccessValue?
		if v == ".*" {
			lf.Value = v
			lf.IsRegexp = true
			return lf, nil
		}
	}

	if len(buffer) == 1 {
		lf.Value = buffer[0]
		// TODO: move to a helper?
		if strings.ContainsAny(lf.Value, `.+*?^$()[]{}|\`) {
			lf.IsRegexp = true
			// Trim anchors as they're not needed for Prometheus, and not expected in the app.shouldBeModified function
			lf.Value = strings.TrimLeft(lf.Value, "^")
			lf.Value = strings.TrimLeft(lf.Value, "(")
			lf.Value = strings.TrimRight(lf.Value, "$")
			lf.Value = strings.TrimRight(lf.Value, ")")
		}
	} else {
		// "Regex matches are fully anchored. A match of env=~"foo" is treated as env=~"^foo$"." https://prometheus.io/docs/prometheus/latest/querying/basics/
		lf.Value = strings.Join(buffer, "|")
		lf.IsRegexp = true
	}

	if lf.IsRegexp {
		_, err := regexp.Compile(lf.Value)
		if err != nil {
			return metricsql.LabelFilter{}, fmt.Errorf("%s in %q (converted from %q)", err, lf.Value, rawACL)
		}
	}

	return lf, nil
}

// loadACL loads ACL from a file
func (app *application) loadACL() (ACLMap, error) {
	aclMap := make(ACLMap)

	yamlFile, err := os.ReadFile(app.ACLPath)
	if err != nil {
		return aclMap, err
	}
	var aclYaml map[string]string

	err = yaml.Unmarshal(yamlFile, &aclYaml)
	if err != nil {
		return aclMap, err
	}

	for role, rawACL := range aclYaml {
		acl := &ACL{}

		lf, err := acl.PrepareLF(rawACL)
		if err != nil {
			return aclMap, err
		}
		acl.Fullaccess = app.HasFullaccessValue(lf.Value)
		acl.LabelFilter = lf
		acl.RawACL = rawACL
		aclMap[role] = acl
	}

	return aclMap, nil
}

// getUserRoles returns a list of role matches between user's claims and the ACLMap.
func (app *application) getUserRoles(oidcRoles []string) ([]string, error) {
	var aclRoles []string

	for _, role := range oidcRoles {
		_, exists := app.ACLMap[role]
		if exists {
			aclRoles = append(aclRoles, role)
		}
	}

	if len(aclRoles) > 0 {
		return aclRoles, nil
	}

	return []string{}, fmt.Errorf("no matching roles found")
}

// TODO: rewrite to also consider fullaccess attribute? Or not needed?
// HasFullaccessValue returns true if a label filter gives access to all namespaces.
func (app *application) HasFullaccessValue(value string) bool {
	return value == ".*"
}

// rolesToRawACL returns a comma-separated list of ACL definitions for all specified roles.
// Basically, it lets you dynamically generate a raw ACL as if it was supplied via acl.yaml
func (app *application) rolesToRawACL(roles []string) string {
	// TODO: rewrite with make?
	// rawACLs := make([]string, 0, len(roles))
	var rawACLs []string

	for _, role := range roles {
		rawACLs = append(rawACLs, app.ACLMap[role].RawACL)
	}

	return strings.Join(rawACLs, ", ")
}

// TODO: return ACL? That would help with deduplication
// getLF returns a label filter associated with a specified list of roles.
func (app *application) getLF(roles []string) (metricsql.LabelFilter, error) {
	if len(roles) == 0 {
		return metricsql.LabelFilter{}, fmt.Errorf("failed to construct a label filter (no roles supplied)")
	}

	if len(roles) == 1 {
		role := roles[0]
		return app.ACLMap[role].LabelFilter, nil
	}

	// If a user has a fullaccess role, there's no need to check any other one.
	for _, role := range roles {
		if app.ACLMap[role].Fullaccess {
			return app.ACLMap[role].LabelFilter, nil
		}
	}

	rawACL := app.rolesToRawACL(roles)

	acl := &ACL{}

	lf, err := acl.PrepareLF(rawACL)
	if err != nil {
		return metricsql.LabelFilter{}, err
	}

	return lf, nil
}
