package querymodifier

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ACLs stores a parsed YAML with role defitions
type ACLs map[string]ACL

// rolesToRawACL returns a comma-separated list of ACL definitions for all specified roles. Basically, it lets you dynamically generate a raw ACL as if it was supplied through acl.yaml. To support Assumed Roles, unknown roles are treated as ACL definitions.
func (a ACLs) rolesToRawACL(roles []string) (string, error) {
	rawACLs := make([]string, 0, len(roles))

	for _, role := range roles {
		acl, exists := a[role]
		if exists {
			// NOTE: You should never see an empty definitions in .RawACL as those should be removed by toSlice further down the process. The error check below is not necessary, is left as an additional safeguard for now and might get removed in the future.
			if acl.RawACL == "" {
				return "", fmt.Errorf("%s role contains empty rawACL", role)
			}
			rawACLs = append(rawACLs, acl.RawACL)
		} else {
			// NOTE: Role names are not linted, so they may contain regular expressions, including the admin definition: .*
			rawACLs = append(rawACLs, role)
		}
	}

	rawACL := strings.Join(rawACLs, ", ")
	if rawACL == "" {
		return "", fmt.Errorf("constructed empty rawACL")
	}

	return rawACL, nil
}

// GetUserACL takes a list of roles found in an OIDC claim and constructs and ACL based on them. If assumed roles are disabled, then only known roles (present in app.ACLs) are considered.
func (a ACLs) GetUserACL(oidcRoles []string, assumedRolesEnabled bool) (ACL, error) {
	roles := []string{}
	assumedRoles := []string{}

	for _, role := range oidcRoles {
		_, exists := a[role]
		if exists {
			if a[role].Fullaccess {
				return a[role], nil
			}
			roles = append(roles, role)
		} else {
			assumedRoles = append(assumedRoles, role)
		}
	}

	if assumedRolesEnabled {
		roles = append(roles, assumedRoles...)
	}

	if len(roles) == 0 {
		return ACL{}, fmt.Errorf("no matching roles found")
	}

	// We can return a prebuilt ACL if there's only one role and it's known
	if len(roles) == 1 {
		role := roles[0]
		acl, exists := a[role]
		if exists {
			return acl, nil
		}
	}

	// To simplify creation of composite ACLs, we need to form a raw ACL, so the further process would be equal to what we have for processing acl.yaml
	rawACL, err := a.rolesToRawACL(roles)
	if err != nil {
		return ACL{}, err
	}

	acl, err := NewACL(rawACL)
	if err != nil {
		return ACL{}, err
	}

	return acl, nil
}

// NewACLsFromFile loads ACL from a file or returns an empty ACLs instance if path is empty
func NewACLsFromFile(path string) (ACLs, error) {
	acls := make(ACLs)

	path = strings.TrimSpace(path)
	if path == "" {
		return acls, nil
	}

	yamlFile, err := os.ReadFile(path)
	if err != nil {
		return ACLs{}, err
	}
	var aclYaml map[string]string

	err = yaml.Unmarshal(yamlFile, &aclYaml)
	if err != nil {
		return ACLs{}, err
	}

	for role, rawACL := range aclYaml {
		acl, err := NewACL(rawACL)
		if err != nil {
			return ACLs{}, err
		}

		acls[role] = acl
	}

	return acls, nil
}
