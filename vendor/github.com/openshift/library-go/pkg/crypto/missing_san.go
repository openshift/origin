package crypto

import (
	"crypto/x509"
	"errors"
	"strings"
)

// CertHasSAN returns true if the given certificate includes a SAN field, else false.
func CertHasSAN(c *x509.Certificate) bool {
	if c == nil {
		return false
	}

	sanOID := []int{2, 5, 29, 17}

	for i := range c.Extensions {
		if c.Extensions[i].Id.Equal(sanOID) {
			return true
		}
	}
	return false
}

// IsHostnameError returns true if the error indicates a host name error about legacy CN fields
// else false as a result of `x509.Certificate#VerifyHostname`.
//
// For Golang <1.17: If GODEBUG=x509ignoreCN=0 is set this will always return false.
// In this case, use `crypto.CertHasSAN` to assert validity of the certificate directly.
//
// See https://github.com/golang/go/blob/go1.16.12/src/crypto/x509/verify.go#L119
func IsHostnameError(err error) bool {
	if err != nil &&
		errors.As(err, &x509.HostnameError{}) &&
		strings.Contains(err.Error(), "x509: certificate relies on legacy Common Name field") {
		return true
	}

	return false
}
