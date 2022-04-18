package querymodifier

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/VictoriaMetrics/metricsql"
)

// RegexpSymbols is used to determine whether ACL definition is a regexp or whether LF contains a fake regexp
const RegexpSymbols = `.+*?^$()[]{}|\`

// ACL stores a role definition
type ACL struct {
	Fullaccess  bool
	LabelFilter metricsql.LabelFilter
	RawACL      string
}

// NewACL returns an ACL based on a rule definition (non-regexp for one namespace, regexp - for many). .RawACL in the resulting value will contain a normalized value (anchors stripped, implicit admin will have only .*).
func NewACL(rawACL string) (ACL, error) {
	var lf = metricsql.LabelFilter{
		Label:      "namespace",
		IsNegative: false,
		IsRegexp:   false,
	}

	buffer, err := toSlice(rawACL)
	if err != nil {
		return ACL{}, err
	}

	// If .* is in the slice, then we can omit any other value
	for _, v := range buffer {
		// TODO: move to a helper?
		if v == ".*" {
			// Note: with this approach, we intentionally omit other values in the resulting ACL
			return getFullaccessACL(), nil
		}
	}

	if len(buffer) == 1 {
		// TODO: move to a helper?
		if strings.ContainsAny(buffer[0], RegexpSymbols) {
			lf.IsRegexp = true
			// Trim anchors as they're not needed for Prometheus, and not expected in the app.shouldBeModified function
			buffer[0] = strings.TrimLeft(buffer[0], "^")
			buffer[0] = strings.TrimLeft(buffer[0], "(")
			buffer[0] = strings.TrimRight(buffer[0], "$")
			buffer[0] = strings.TrimRight(buffer[0], ")")
		}
		lf.Value = buffer[0]
	} else {
		// "Regex matches are fully anchored. A match of env=~"foo" is treated as env=~"^foo$"." https://prometheus.io/docs/prometheus/latest/querying/basics/
		lf.Value = strings.Join(buffer, "|")
		lf.IsRegexp = true
	}

	if lf.IsRegexp {
		_, err := regexp.Compile(lf.Value)
		if err != nil {
			return ACL{}, fmt.Errorf("%s in %q (converted from %q)", err, lf.Value, rawACL)
		}
	}

	acl := ACL{
		Fullaccess:  false,
		LabelFilter: lf,
		RawACL:      strings.Join(buffer, ", "),
	}

	return acl, nil
}

// getFullaccessACL returns a fullaccess ACL
func getFullaccessACL() ACL {
	return ACL{
		Fullaccess: true,
		LabelFilter: metricsql.LabelFilter{
			Label:      "namespace",
			Value:      ".*",
			IsRegexp:   true,
			IsNegative: false,
		},
		RawACL: ".*",
	}
}
