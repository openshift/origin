package ldaputil

import (
	"crypto/tls"
	"fmt"
	"net"

	"k8s.io/client-go/util/cert"

	"github.com/openshift/origin/pkg/auth/ldaputil/ldapclient"
	"gopkg.in/ldap.v2"
)

// NewLDAPClientConfig returns a new LDAP client config
func NewLDAPClientConfig(URL, bindDN, bindPassword, CA string, insecure bool) (ldapclient.Config, error) {
	url, err := ParseURL(URL)
	if err != nil {
		return nil, fmt.Errorf("Error parsing URL: %v", err)
	}

	tlsConfig := &tls.Config{}
	if len(CA) > 0 {
		roots, err := cert.NewPool(CA)
		if err != nil {
			return nil, fmt.Errorf("error loading cert pool from ca file %s: %v", CA, err)
		}
		tlsConfig.RootCAs = roots
	}

	return &ldapClientConfig{
		scheme:       url.Scheme,
		host:         url.Host,
		bindDN:       bindDN,
		bindPassword: bindPassword,
		insecure:     insecure,
		tlsConfig:    tlsConfig,
	}, nil
}

// ldapClientConfig holds information for connecting to an LDAP server
type ldapClientConfig struct {
	// scheme is the LDAP connection scheme, either ldap or ldaps
	scheme Scheme
	// host is the host:port of the LDAP server
	host string
	// bindDN is an optional DN to bind with during the search phase.
	bindDN string
	// bindPassword is an optional password to bind with during the search phase.
	bindPassword string
	// insecure specifies if TLS is required for the connection. If true, either an ldap://... URL or
	// StartTLS must be supported by the server
	insecure bool
	// tlsConfig holds the TLS options. Only used when insecure=false
	tlsConfig *tls.Config
}

// ldapClientConfig is an ldapclient.Config
var _ ldapclient.Config = &ldapClientConfig{}

// Connect returns an established LDAP connection, or an error if the connection could not
// be made (or successfully upgraded to TLS). If no error is returned, the caller is responsible for
// closing the connection
func (l *ldapClientConfig) Connect() (ldap.Client, error) {
	tlsConfig := l.tlsConfig

	// Ensure tlsConfig specifies the server we're connecting to
	if tlsConfig != nil && !tlsConfig.InsecureSkipVerify && len(tlsConfig.ServerName) == 0 {
		// Add to a copy of the tlsConfig to avoid mutating the original
		c := tlsConfig.Clone()
		if host, _, err := net.SplitHostPort(l.host); err == nil {
			c.ServerName = host
		} else {
			c.ServerName = l.host
		}
		tlsConfig = c
	}

	switch l.scheme {
	case SchemeLDAP:
		con, err := ldap.Dial("tcp", l.host)
		if err != nil {
			return nil, err
		}

		// If an insecure connection is desired, we're done
		if l.insecure {
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
		return ldap.DialTLS("tcp", l.host, tlsConfig)

	default:
		return nil, fmt.Errorf("unsupported scheme %q", l.scheme)
	}
}

func (l *ldapClientConfig) GetBindCredentials() (string, string) {
	return l.bindDN, l.bindPassword
}

func (l *ldapClientConfig) Host() string {
	return l.host
}

// String implements Stringer for debugging purposes
func (l *ldapClientConfig) String() string {
	return fmt.Sprintf("{Scheme: %v Host: %v BindDN: %v len(BbindPassword): %v Insecure: %v}", l.scheme, l.host, l.bindDN, len(l.bindPassword), l.insecure)
}
