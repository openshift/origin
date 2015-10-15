package ldaputil

import (
	"crypto/tls"
	"fmt"
	"net"

	"k8s.io/kubernetes/pkg/util"

	"github.com/go-ldap/ldap"
)

// NewLDAPClientConfig returns a new LDAPClientConfig
func NewLDAPClientConfig(URL, bindDN, bindPassword, CA string, insecure bool) (*LDAPClientConfig, error) {
	url, err := ParseURL(URL)
	if err != nil {
		return nil, fmt.Errorf("Error parsing URL: %v", err)
	}

	tlsConfig := &tls.Config{}
	if len(CA) > 0 {
		roots, err := util.CertPoolFromFile(CA)
		if err != nil {
			return nil, fmt.Errorf("error loading cert pool from ca file %s: %v", CA, err)
		}
		tlsConfig.RootCAs = roots
	}

	return &LDAPClientConfig{
		Scheme:       url.Scheme,
		Host:         url.Host,
		BindDN:       bindDN,
		BindPassword: bindPassword,
		Insecure:     insecure,
		TLSConfig:    tlsConfig,
	}, nil
}

// LDAPClientConfig holds information for connecting to an LDAP server
type LDAPClientConfig struct {
	// Scheme is the LDAP connection scheme, either ldap or ldaps
	Scheme Scheme
	// Host is the host:port of the LDAP server
	Host string
	// BindDN is an optional DN to bind with during the search phase.
	BindDN string
	// BindPassword is an optional password to bind with during the search phase.
	BindPassword string
	// Insecure specifies if TLS is required for the connection. If true, either an ldap://... URL or
	// StartTLS must be supported by the server
	Insecure bool
	// TLSConfig holds the TLS options. Only used when Insecure=false
	TLSConfig *tls.Config
}

func (l LDAPClientConfig) String() string {
	return fmt.Sprintf("{Scheme: %v Host: %v BindDN: %v len(BindPassword: %v Insecure: %v}", l.Scheme, l.Host, l.BindDN, len(l.BindPassword), l.Insecure)
}

// Connect returns an established LDAP connection, or an error if the connection could not
// be made (or successfully upgraded to TLS). If no error is returned, the caller is responsible for
// closing the connection
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

// Bind binds to a given LDAP connection if a bind DN and password were given.
// Bind returns whether a bind occured and whether an error occurred
func (l *LDAPClientConfig) Bind(connection *ldap.Conn) (bound bool, err error) {
	if len(l.BindDN) > 0 {
		if err := connection.Bind(l.BindDN, l.BindPassword); err != nil {
			return false, err
		} else {
			return true, nil
		}
	}

	return false, nil
}
