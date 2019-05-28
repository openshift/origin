package ldappassword

import (
	"fmt"

	"gopkg.in/ldap.v2"

	"k8s.io/apimachinery/pkg/util/sets"

	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/security/ldaputil"
	authapi "github.com/openshift/oauth-server/pkg/api"
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

func NewLDAPUserAttributeDefiner(attributeMapping osinv1.LDAPAttributeMapping) LDAPUserAttributeDefiner {
	return LDAPUserAttributeDefiner{
		attributeMapping: attributeMapping,
	}
}

// LDAPUserAttributeDefiner defines the values corresponding to OpenShift Identities in LDAP entries
// by using a deterministic mapping of LDAP entry attributes to OpenShift Identity fields
type LDAPUserAttributeDefiner struct {
	// attributeMapping holds the attributes mapped to email, name, preferred username and ID
	attributeMapping osinv1.LDAPAttributeMapping
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
	return ldaputil.GetAttributeValue(user, d.attributeMapping.Email)
}

// Name extracts the name value from an LDAP user entry
func (d *LDAPUserAttributeDefiner) Name(user *ldap.Entry) string {
	return ldaputil.GetAttributeValue(user, d.attributeMapping.Name)
}

// PreferredUsername extracts the preferred username value from an LDAP user entry
func (d *LDAPUserAttributeDefiner) PreferredUsername(user *ldap.Entry) string {
	return ldaputil.GetAttributeValue(user, d.attributeMapping.PreferredUsername)
}

// ID extracts the ID value from an LDAP user entry
func (d *LDAPUserAttributeDefiner) ID(user *ldap.Entry) string {
	// support binary ID fields as those the only stable identifiers in some environments
	return ldaputil.GetRawAttributeValue(user, d.attributeMapping.ID)
}
