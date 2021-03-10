package main

import (
	"fmt"
	"io/ioutil"
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
}

// toSlice converts namespace rules to string slices.
func (a *ACL) toSlice(str string) ([]string, error) {
	buffer := []string{}
	// yaml values should come already with trimmed leading and trailing spaces
	for _, s := range strings.Split(str, ",") {
		// in case there are empty elements in between
		s := strings.TrimSpace(s)

		for _, ch := range s {
			if unicode.IsSpace(ch) {
				return nil, fmt.Errorf("Line should not contain spaces within individual elements (%q)", str)
			}
		}

		if s != "" {
			buffer = append(buffer, s)
		}
	}

	if len(buffer) == 0 {
		return nil, fmt.Errorf("Line has to contain at least one valid element (%q)", str)
	}

	return buffer, nil
}

// PrepareLF returns a label filter based on a rule definition (non-regexp for one namespace, regexp - for many)
func (a *ACL) PrepareLF(ns string) (metricsql.LabelFilter, error) {
	var lf = metricsql.LabelFilter{
		Label:      "namespace",
		IsNegative: false,
	}

	if ns == ".*" {
		lf.Value = ns
		lf.IsRegexp = true
	}

	buffer, err := a.toSlice(ns)
	if err != nil {
		return metricsql.LabelFilter{}, err
	}

	if len(buffer) == 1 {
		lf.Value = buffer[0]
		if strings.ContainsAny(buffer[0], `.+*?^$()[]{}|\`) {
			lf.IsRegexp = true
		}
	} else {
		lf.Value = fmt.Sprintf("^(%s)$", strings.Join(buffer, "|"))
		lf.IsRegexp = true
	}

	if lf.IsRegexp {
		_, err := regexp.Compile(lf.Value)
		if err != nil {
			return metricsql.LabelFilter{}, fmt.Errorf("%s in %q (converted from %q)", err, lf.Value, ns)
		}
	}

	return lf, nil
}

func (app *application) loadACL() (ACLMap, error) {
	aclMap := make(ACLMap)

	yamlFile, err := ioutil.ReadFile(app.ACLPath)
	if err != nil {
		return aclMap, err
	}
	var aclYaml map[string]string

	err = yaml.Unmarshal(yamlFile, &aclYaml)
	if err != nil {
		app.errorLog.Fatal(err)
	}

	for role, ns := range aclYaml {
		acl := &ACL{}
		if ns == ".*" {
			acl.Fullaccess = true
		}

		lf, err := acl.PrepareLF(ns)
		if err != nil {
			return aclMap, err
		}
		acl.LabelFilter = lf
		aclMap[role] = acl
		app.infoLog.Printf("Loaded role definition for %s: %q (converted to %s)", role, ns, acl.LabelFilter.AppendString(nil))
	}

	return aclMap, nil
}

// getUserRole returns a first role match between user's claims and the ACLMap.
func (app *application) getUserRole(roles []string) (string, error) {
	for _, role := range roles {
		_, exists := app.ACLMap[role]
		if exists {
			return role, nil
		}
	}
	return "", fmt.Errorf("No matching role found")
}

// hasFullaccessRole says whether a role has offers a full access as per an acl spec.
func (app *application) hasFullaccessRole(role string) bool {
	return app.ACLMap[role].Fullaccess
}

// getLF returns a label filter associated with a specified role.
func (app *application) getLF(role string) metricsql.LabelFilter {
	return app.ACLMap[role].LabelFilter
}
