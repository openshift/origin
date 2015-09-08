package ldaputil

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/go-ldap/ldap"
)

// NewLDAPClientConfig returns a new LDAPClientConfig
func NewLDAPClientConfig(url LDAPURL, insecure bool, tlsConfig *tls.Config) LDAPClientConfig {
	return LDAPClientConfig{
		Scheme:    url.Scheme,
		Host:      url.Host,
		Insecure:  insecure,
		TLSConfig: tlsConfig,
	}
}

// LDAPClientConfig holds information for connecting to an LDAP server
type LDAPClientConfig struct {
	// Scheme is ldap or ldaps
	Scheme Scheme
	// Host is the host:port of the LDAP server
	Host string
	// Insecure specifies if TLS is required for the connection. If true, either an ldap://... URL or
	// StartTLS must be supported by the server
	Insecure bool
	// TLSConfig holds the TLS options. Only used when Insecure=false
	TLSConfig *tls.Config
}

// Connect returns an established LDAP connection, or an error if the connection could not be made
// (or successfully upgraded to TLS). If no error is returned, the caller is responsible for closing
// the connection
func (l *LDAPClientConfig) Connect() (*ldap.Conn, error) {
	tlsConfig := l.TLSConfig

	// Ensure tlsConfig specifies the server we're connecting to
	if tlsConfig != nil && !tlsConfig.InsecureSkipVerify && len(tlsConfig.ServerName) == 0 {
		// Add to a copy of the tlsConfig to avoid mutating the original
		c := *tlsConfig
		if host, _, err := net.SplitHostPort(l.Host); err == nil {
			c.ServerName = host
		} else {
			c.ServerName = l.Host
		}
		tlsConfig = &c
	}

	switch l.Scheme {
	case SchemeLDAP:
		con, err := ldap.Dial("tcp", l.Host)
		if err != nil {
			return nil, err
		}

		// If an insecure connection is desired, we're done
		if l.Insecure {
			return con, nil
		}

		// Attempt to upgrade to TLS
		if err := con.StartTLS(tlsConfig); err != nil {
			// We're returning an error on a successfully opened connection
			// We are responsible for closing the open connection
			con.Close()
			return nil, err
		}

		return con, nil

	case SchemeLDAPS:
		return ldap.DialTLS("tcp", l.Host, tlsConfig)

	default:
		return nil, fmt.Errorf("unsupported scheme %q", l.Scheme)
	}
}
