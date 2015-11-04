package ldapclient

import "github.com/go-ldap/ldap"

// Config knows how to connect to an LDAP server and can describe which server it is connecting to
type Config interface {
	Connect() (client ldap.Client, err error)
	GetBindCredentials() (bindDN, bindPassword string)
	Host() string
}
