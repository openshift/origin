package testclient

import (
	"github.com/go-ldap/ldap"
	"github.com/openshift/origin/pkg/auth/ldaputil/ldapclient"
)

// fakeConfig regurgitates internal state in order to conform to Config
type fakeConfig struct {
	client ldap.Client
}

// NewConfig creates a new Config impl that regurgitates the given data
func NewConfig(client ldap.Client) ldapclient.Config {
	return &fakeConfig{
		client: client,
	}
}

func (c *fakeConfig) Connect() (ldap.Client, error) {
	return c.client, nil
}

func (c *fakeConfig) GetBindCredentials() (string, string) {
	return "", ""
}

func (c *fakeConfig) Host() string {
	return ""
}
