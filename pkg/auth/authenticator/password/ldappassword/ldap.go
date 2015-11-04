package ldappassword

import (
	"fmt"
	"runtime/debug"

	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/go-ldap/ldap"
	"github.com/golang/glog"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/auth/ldaputil/ldapclient"
)

// Options contains configuration for an Authenticator instance
type Options struct {
	// URL is a parsed RFC 2255 URL
	URL ldaputil.LDAPURL
	// ClientConfig holds information about connecting with the LDAP server
	ClientConfig ldapclient.Config

	// UserAttributeDefiner defines the values corresponding to OpenShift Identities in LDAP entries
	// by using a deterministic mapping of LDAP entry attributes to OpenShift Identity fields. The first
	// attribute with a non-empty value is used for all but the latter identity field. If no LDAP attributes
	// are given for the ID address, login fails.
	UserAttributeDefiner ldaputil.LDAPUserAttributeDefiner
}

// Authenticator validates username/passwords against an LDAP v3 server
type Authenticator struct {
	providerName    string
	options         Options
	mapper          authapi.UserIdentityMapper
	identityFactory ldaputil.LDAPUserIdentityFactory
}

// New returns an authenticator which will validate usernames/passwords using LDAP.
func New(providerName string, options Options, mapper authapi.UserIdentityMapper) (authenticator.Password, error) {
	auth := &Authenticator{
		providerName: providerName,
		options:      options,
		mapper:       mapper,
		identityFactory: &ldaputil.DefaultLDAPUserIdentityFactory{
			ProviderName: providerName,
			Definer:      options.UserAttributeDefiner,
		},
	}
	return auth, nil
}

// AuthenticatePassword validates the given username and password against an LDAP server
func (a *Authenticator) AuthenticatePassword(username, password string) (user.Info, bool, error) {
	identity, ok, err := a.getIdentity(username, password)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	user, err := a.mapper.UserFor(identity)
	if err != nil {
		return nil, false, fmt.Errorf("Error creating or updating mapping for: %#v due to %v", identity, err)
	}
	glog.V(4).Infof("Got userIdentityMapping: %#v", user)

	return user, true, nil

}

// getIdentity looks up a username in an LDAP server, and attempts to bind to the user's DN using the provided password
func (a *Authenticator) getIdentity(username, password string) (authapi.UserIdentityInfo, bool, error) {
	defer func() {
		if e := recover(); e != nil {
			util.HandleError(fmt.Errorf("Recovered panic: %v, %s", e, debug.Stack()))
		}
	}()

	if len(username) == 0 || len(password) == 0 {
		return nil, false, nil
	}

	// Make the connection and bind to it if a bind DN and password were given
	l, err := a.options.ClientConfig.Connect()
	if err != nil {
		return nil, false, err
	}
	defer l.Close()

	if bindDN, bindPassword := a.options.ClientConfig.GetBindCredentials(); len(bindDN) > 0 {
		if err := l.Bind(bindDN, bindPassword); err != nil {
			return nil, false, err
		}
	}

	// & together the filter specified in the LDAP options with the user-specific filter
	filter := fmt.Sprintf("(&%s(%s=%s))",
		a.options.URL.Filter,
		ldap.EscapeFilter(a.options.URL.QueryAttribute),
		ldap.EscapeFilter(username),
	)

	// Build list of attributes to retrieve
	attrs := sets.NewString(a.options.URL.QueryAttribute)
	attrs.Insert(a.options.UserAttributeDefiner.AllAttributes().List()...)

	// Search for LDAP record
	searchRequest := ldap.NewSearchRequest(
		a.options.URL.BaseDN,     // base dn
		int(a.options.URL.Scope), // scope
		ldap.NeverDerefAliases,   // deref
		2,            // size limit, we want to know if this is not unique, but don't want the entire tree
		0,            // no client-specified time limit, determined by LDAP server. TODO: make configurable?
		false,        // not types only
		filter,       // filter
		attrs.List(), // attributes to retrieve
		nil,          // controls
	)

	glog.V(4).Infof("searching for %s", filter)
	results, err := l.Search(searchRequest)
	if err != nil {
		return nil, false, err
	}

	if len(results.Entries) == 0 {
		// 0 results means a missing username, not an error
		glog.V(4).Infof("no entries matching %s", filter)
		return nil, false, nil
	}
	if len(results.Entries) > 1 {
		// More than 1 result means a misconfigured server filter or query parameter
		return nil, false, fmt.Errorf("multiple entries found matching %q", username)
	}

	entry := results.Entries[0]
	glog.V(4).Infof("found dn=%q for %s", entry.DN, filter)

	// Bind with given username and password to attempt to authenticate
	if err := l.Bind(entry.DN, password); err != nil {
		glog.V(4).Infof("error binding password for %q: %v", entry.DN, err)
		if err, ok := err.(*ldap.Error); ok {
			switch err.ResultCode {
			case ldap.LDAPResultInappropriateAuthentication:
				// inappropriateAuthentication (48)
				//    Indicates the server requires the client that had attempted
				//    to bind anonymously or without supplying credentials to
				//    provide some form of credentials.
				fallthrough
			case ldap.LDAPResultInvalidCredentials:
				// invalidCredentials (49)
				//    Indicates that the provided credentials (e.g., the user's name
				//    and password) are invalid.

				// Authentication failed, return false, but no error
				return nil, false, nil
			}
		}
		return nil, false, err
	}

	// Build the identity
	identity, err := a.identityFactory.IdentityFor(entry)
	if err != nil {
		return nil, false, err
	}
	return identity, true, nil
}
