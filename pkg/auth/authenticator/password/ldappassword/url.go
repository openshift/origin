package ldappassword

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-ldap/ldap"
)

// Scheme is a valid ldap scheme
type Scheme string

const (
	SchemeLDAP  Scheme = "ldap"
	SchemeLDAPS Scheme = "ldaps"
)

// Scope is a valid LDAP search scope
type Scope int

const (
	ScopeWholeSubtree Scope = ldap.ScopeWholeSubtree
	ScopeSingleLevel  Scope = ldap.ScopeSingleLevel
	ScopeBaseObject   Scope = ldap.ScopeBaseObject
)

const (
	defaultLDAPPort  = 389
	defaultLDAPSPort = 636

	defaultHost           = "localhost"
	defaultQueryAttribute = "uid"
	defaultFilter         = "(objectClass=*)"

	scopeWholeSubtreeString = "sub"
	scopeSingleLevelString  = "one"
	scopeBaseObjectString   = "base"

	criticalExtensionPrefix = "!"
)

// LDAPURL holds a parsed RFC 2255 URL
type LDAPURL struct {
	// Scheme is ldap or ldaps
	Scheme Scheme
	// Host is the host:port of the LDAP server
	Host string
	// The DN of the branch of the directory where all searches should start from
	BaseDN string
	// The attribute to search for
	QueryAttribute string
	// The scope of the search. Can be ldap.ScopeWholeSubtree, ldap.ScopeSingleLevel, or ldap.ScopeBaseObject
	Scope Scope
	// A valid LDAP search filter (e.g. "(objectClass=*)")
	Filter string
}

// ParseURL parsed the given ldapURL as an RFC 2255 URL
// The syntax of the URL is ldap://host:port/basedn?attribute?scope?filter
func ParseURL(ldapURL string) (LDAPURL, error) {
	// Must be a valid URL to start
	parsedURL, err := url.Parse(ldapURL)
	if err != nil {
		return LDAPURL{}, err
	}

	opts := LDAPURL{}

	// Set scheme (default to ldap)
	opts.Scheme = Scheme(parsedURL.Scheme)
	switch opts.Scheme {
	case SchemeLDAP, SchemeLDAPS:
		// ok
	default:
		return LDAPURL{}, fmt.Errorf("invalid scheme %q", parsedURL.Scheme)
	}

	// Set host (default to localhost to match mod_auth_ldap)
	opts.Host = parsedURL.Host
	if len(opts.Host) == 0 {
		opts.Host = defaultHost
	}

	// Add port if needed
	if _, _, err := net.SplitHostPort(opts.Host); err != nil {
		switch opts.Scheme {
		case SchemeLDAPS:
			opts.Host = net.JoinHostPort(opts.Host, strconv.Itoa(defaultLDAPSPort))
		case SchemeLDAP:
			opts.Host = net.JoinHostPort(opts.Host, strconv.Itoa(defaultLDAPPort))
		default:
			return LDAPURL{}, fmt.Errorf("no default port for scheme %q", opts.Scheme)
		}
	}

	// Set base dn (default to "")
	// url.Parse() already percent-decodes the path
	opts.BaseDN = strings.TrimLeft(parsedURL.Path, "/")

	// Split query
	// All sections are optional
	// attribute?scope?filter?extensions
	var attributes, scope, filter, extensions string
	parts := strings.Split(parsedURL.RawQuery, "?")
	switch len(parts) {
	case 4:
		extensions = parts[3]
		fallthrough
	case 3:
		if v, err := url.QueryUnescape(parts[2]); err != nil {
			return LDAPURL{}, err
		} else {
			filter = v
		}
		fallthrough
	case 2:
		if v, err := url.QueryUnescape(parts[1]); err != nil {
			return LDAPURL{}, err
		} else {
			scope = v
		}
		fallthrough
	case 1:
		if v, err := url.QueryUnescape(parts[0]); err != nil {
			return LDAPURL{}, err
		} else {
			attributes = v
		}
	case 0:
		// no-op
	default:
		return LDAPURL{}, fmt.Errorf("too many query options %q", parsedURL.RawQuery)
	}

	// Attributes contains comma-separated attributes
	// Set query attribute to first attribute
	// Default to uid to match mod_auth_ldap
	opts.QueryAttribute = strings.Split(attributes, ",")[0]
	if len(opts.QueryAttribute) == 0 {
		opts.QueryAttribute = defaultQueryAttribute
	}

	// Scope is one of "sub", "one", or "base"
	// Default to "sub" to match mod_auth_ldap
	switch scope {
	case "", scopeWholeSubtreeString:
		opts.Scope = ScopeWholeSubtree
	case scopeSingleLevelString:
		opts.Scope = ScopeSingleLevel
	case scopeBaseObjectString:
		opts.Scope = ScopeBaseObject
	default:
		return LDAPURL{}, fmt.Errorf("invalid scope %q", scope)
	}

	// Filter is a valid LDAP filter
	// Default to "(objectClass=*)" per RFC
	opts.Filter = filter
	if len(opts.Filter) == 0 {
		opts.Filter = defaultFilter
	}
	if _, err := ldap.CompileFilter(opts.Filter); err != nil {
		return LDAPURL{}, fmt.Errorf("invalid filter: %v", err)
	}

	// Extensions are in "name=value,name2=value2" form
	// Critical extensions are prefixed with a !
	// Optional extensions are ignored, per RFC
	// Fail if there are any critical extensions, since we don't support any
	if len(extensions) > 0 {
		for _, extension := range strings.Split(extensions, ",") {
			exttype := strings.SplitN(extension, "=", 2)[0]
			if strings.HasPrefix(exttype, criticalExtensionPrefix) {
				return LDAPURL{}, fmt.Errorf("unsupported critical extension %s", extension)
			}
		}
	}

	return opts, nil

}
