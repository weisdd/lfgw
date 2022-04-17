package acl

import (
	"fmt"
	"strings"
	"unicode"
)

// toSlice converts namespace rules to string slices.
func toSlice(str string) ([]string, error) {
	buffer := []string{}
	// yaml values should come already with trimmed leading and trailing spaces
	for _, s := range strings.Split(str, ",") {
		// in case there are empty elements in between
		s := strings.TrimSpace(s)

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
