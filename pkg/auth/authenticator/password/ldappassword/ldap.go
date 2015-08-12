package ldappassword

import (
	"crypto/tls"
	"fmt"
	"net"
	"runtime/debug"
	"strings"

	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util"

	"github.com/go-ldap/ldap"
	"github.com/golang/glog"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
)

// Options contains configuration for an Authenticator instance
type Options struct {
	// URL is a parsed RFC 2255 URL
	URL LDAPURL
	// Insecure specifies if TLS is required for the connection. If true, either an ldap://... URL or StartTLS must be supported by the server
	Insecure bool
	// TLSConfig holds the TLS options. Only used when Insecure=false
	TLSConfig *tls.Config

	// BindDN is the optional username to bind to for the search phase. If specified, BindPassword must also be set.
	BindDN string
	// BindPassword is the optional password to bind to for the search phase.
	BindPassword string

	// AttributeEmail is the optional list of LDAP attributes to use for the email address of the user identity.
	// The first attribute with a non-empty value is used.
	AttributeEmail []string
	// AttributeName is the optional list of LDAP attributes to use for the display name of the user identity.
	// The first attribute with a non-empty value is used.
	AttributeName []string
	// AttributePreferredUsername is the optional list of LDAP attributes to use for the preferred username of the user identity.
	// The first attribute with a non-empty value is used. If not specified, the id determined by AttributeID is used as the preferred login.
	AttributePreferredUsername []string
	// AttributeID is the required list of LDAP attributes to use for the id address of the user identity.
	// The first attribute with a non-empty value is used. If no attributes have values, login fails.
	AttributeID []string
}

// Authenticator validates username/passwords against an LDAP v3 server
type Authenticator struct {
	providerName string
	options      Options
	mapper       authapi.UserIdentityMapper
}

// New returns an authenticator which will validate usernames/passwords using LDAP.
func New(providerName string, options Options, mapper authapi.UserIdentityMapper) (authenticator.Password, error) {
	auth := &Authenticator{
		providerName: providerName,
		options:      options,
		mapper:       mapper,
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
	glog.V(4).Infof("Got userIdentityMapping: %#v", user)
	if err != nil {
		return nil, false, fmt.Errorf("Error creating or updating mapping for: %#v due to %v", identity, err)
	}

	return user, true, nil

}

// connect returns an established ldap connection, or an error if the connection could not be made (or successfully upgraded to TLS)
// if no error is returned, the caller is responsible for closing the connection
func (a *Authenticator) connect() (*ldap.Conn, error) {
	tlsConfig := a.options.TLSConfig

	// Ensure tlsConfig specifies the server we're connecting to
	if tlsConfig != nil && !tlsConfig.InsecureSkipVerify && len(tlsConfig.ServerName) == 0 {
		// Add to a copy of the tlsConfig to avoid mutating the original
		c := *tlsConfig
		if host, _, err := net.SplitHostPort(a.options.URL.Host); err == nil {
			c.ServerName = host
		} else {
			c.ServerName = a.options.URL.Host
		}
		tlsConfig = &c
	}

	switch a.options.URL.Scheme {
	case SchemeLDAP:
		l, err := ldap.Dial("tcp", a.options.URL.Host)
		if err != nil {
			return nil, err
		}

		// If an insecure connection is desired, we're done
		if a.options.Insecure {
			return l, nil
		}

		// Attempt to upgrade to TLS
		if err := l.StartTLS(tlsConfig); err != nil {
			// We're returning an error on a successfully opened connection
			// We are responsible for closing the open connection
			l.Close()
			return nil, err
		}

		return l, nil

	case SchemeLDAPS:
		return ldap.DialTLS("tcp", a.options.URL.Host, tlsConfig)

	default:
		return nil, fmt.Errorf("unsupported scheme %q", a.options.URL.Scheme)
	}
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

	// Make the connection
	l, err := a.connect()
	if err != nil {
		return nil, false, err
	}
	defer l.Close()

	// If specified, bind the username/password for search phase
	if len(a.options.BindDN) > 0 {
		if err := l.Bind(a.options.BindDN, a.options.BindPassword); err != nil {
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
	attrs := util.NewStringSet(a.options.URL.QueryAttribute)
	attrs.Insert(a.options.AttributeEmail...)
	attrs.Insert(a.options.AttributeName...)
	attrs.Insert(a.options.AttributePreferredUsername...)
	attrs.Insert(a.options.AttributeID...)

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
	uid := getAttributeValue(entry, a.options.AttributeID)
	if uid == "" {
		return nil, false, fmt.Errorf("Could not retrieve a non-empty value from %v attributes for dn=%q", a.options.AttributeID, entry.DN)
	}
	identity := authapi.NewDefaultUserIdentityInfo(a.providerName, uid)

	// Add optional extra attributes if present
	for k, attrs := range map[string][]string{
		authapi.IdentityPreferredUsernameKey: a.options.AttributePreferredUsername,
		authapi.IdentityEmailKey:             a.options.AttributeEmail,
		authapi.IdentityDisplayNameKey:       a.options.AttributeName,
	} {
		if v := getAttributeValue(entry, attrs); len(v) != 0 {
			identity.Extra[k] = v
		}
	}

	return identity, true, nil
}

// getValue returns the first non-empty value the entry has for the given attributes
func getAttributeValue(entry *ldap.Entry, attributes []string) string {
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
