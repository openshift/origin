package ldaputil

import (
	"fmt"
	"strings"

	"gopkg.in/ldap.v2"

	"k8s.io/kubernetes/pkg/util/sets"

	authapi "github.com/openshift/origin/pkg/auth/api"
	serverapi "github.com/openshift/origin/pkg/cmd/server/api"
)

// LDAPUserIdentityFactory creates Identites for LDAP user entries.
type LDAPUserIdentityFactory interface {
	IdentityFor(user *ldap.Entry) (identity authapi.UserIdentityInfo, err error)
}

// DefaultLDAPUserIdentityFactory creates Identities for LDAP user entries using an LDAPUserAttributeDefiner
type DefaultLDAPUserIdentityFactory struct {
	ProviderName string
	Definer      LDAPUserAttributeDefiner
}

func (f *DefaultLDAPUserIdentityFactory) IdentityFor(user *ldap.Entry) (identity authapi.UserIdentityInfo, err error) {
	uid := f.Definer.ID(user)
	if uid == "" {
		err = fmt.Errorf("Could not retrieve a non-empty value for ID attributes for dn=%q", user.DN)
		return
	}
	id := authapi.NewDefaultUserIdentityInfo(f.ProviderName, uid)

	// Add optional extra attributes if present
	if name := f.Definer.Name(user); len(name) != 0 {
		id.Extra[authapi.IdentityDisplayNameKey] = name
	}

	if email := f.Definer.Email(user); len(email) != 0 {
		id.Extra[authapi.IdentityEmailKey] = email
	}

	if prefUser := f.Definer.PreferredUsername(user); len(prefUser) != 0 {
		id.Extra[authapi.IdentityPreferredUsernameKey] = prefUser
	}

	identity = id
	return
}

func NewLDAPUserAttributeDefiner(attributeMapping serverapi.LDAPAttributeMapping) LDAPUserAttributeDefiner {
	return LDAPUserAttributeDefiner{
		attributeMapping: attributeMapping,
	}
}

// LDAPUserAttributeDefiner defines the values corresponding to OpenShift Identities in LDAP entries
// by using a deterministic mapping of LDAP entry attributes to OpenShift Identity fields
type LDAPUserAttributeDefiner struct {
	// attributeMapping holds the attributes mapped to email, name, preferred username and ID
	attributeMapping serverapi.LDAPAttributeMapping
}

// AllAttributes gets all attributes listed in the LDAPUserAttributeDefiner
func (d *LDAPUserAttributeDefiner) AllAttributes() sets.String {
	attrs := sets.NewString(d.attributeMapping.Email...)
	attrs.Insert(d.attributeMapping.Name...)
	attrs.Insert(d.attributeMapping.PreferredUsername...)
	attrs.Insert(d.attributeMapping.ID...)
	return attrs
}

// Email extracts the email value from an LDAP user entry
func (d *LDAPUserAttributeDefiner) Email(user *ldap.Entry) string {
	return GetAttributeValue(user, d.attributeMapping.Email)
}

// Name extracts the name value from an LDAP user entry
func (d *LDAPUserAttributeDefiner) Name(user *ldap.Entry) string {
	return GetAttributeValue(user, d.attributeMapping.Name)
}

// PreferredUsername extracts the preferred username value from an LDAP user entry
func (d *LDAPUserAttributeDefiner) PreferredUsername(user *ldap.Entry) string {
	return GetAttributeValue(user, d.attributeMapping.PreferredUsername)
}

// ID extracts the ID value from an LDAP user entry
func (d *LDAPUserAttributeDefiner) ID(user *ldap.Entry) string {
	return GetAttributeValue(user, d.attributeMapping.ID)
}

// GetAttributeValue finds the first attribute of those given that the LDAP entry has, and
// returns it. GetAttributeValue is able to query the DN as well as Attributes of the LDAP entry.
// If no value is found, the empty string is returned.
func GetAttributeValue(entry *ldap.Entry, attributes []string) string {
	for _, k := range attributes {
		// Ignore empty attributes
		if len(k) == 0 {
			continue
		}
		// Special-case DN, since it's not an attribute
		if strings.ToLower(k) == "dn" {
			return entry.DN
		}
		// Otherwise get an attribute and return it if present
		if v := entry.GetAttributeValue(k); len(v) > 0 {
			return v
		}
	}
	return ""
}
