package certs

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"net"
	"testing"
	"time"
)

func init() {
	nowFn = func() time.Time {
		return time.Date(2019, time.January, 1, 0, 0, 0, 0, &time.Location{})
	}
}

func TestCertificateToString(t *testing.T) {
	tests := []struct {
		name     string
		cert     *x509.Certificate
		expected string
	}{
		{
			name:     "empty cert",
			cert:     &x509.Certificate{},
			expected: `"" [] issuer="<self-signed>" (Jan 1 00:00:00 0001 to Jan 1 00:00:00 0001 (now=Jan 1 00:00:00 2019))`,
		},
		{
			name: "common name",
			cert: &x509.Certificate{
				Subject: pkix.Name{
					CommonName: "test-subject",
				},
				Issuer: pkix.Name{
					CommonName: "test-issuer",
				},
			},
			expected: `"test-subject" [] issuer="test-issuer" (Jan 1 00:00:00 0001 to Jan 1 00:00:00 0001 (now=Jan 1 00:00:00 2019))`,
		},
		{
			name: "self-signed",
			cert: &x509.Certificate{
				Subject: pkix.Name{
					CommonName: "test-issuer",
				},
				Issuer: pkix.Name{
					CommonName: "test-issuer",
				},
			},
			expected: `"test-issuer" [] issuer="<self-signed>" (Jan 1 00:00:00 0001 to Jan 1 00:00:00 0001 (now=Jan 1 00:00:00 2019))`,
		},
		{
			name: "valid serving for",
			cert: &x509.Certificate{
				Subject: pkix.Name{
					CommonName: "test-subject",
				},
				Issuer: pkix.Name{
					CommonName: "test-issuer",
				},
				IPAddresses: []net.IP{net.IPv4('1', '2', '3', '4')},
			},
			expected: `"test-subject" [] validServingFor=[49.50.51.52] issuer="test-issuer" (Jan 1 00:00:00 0001 to Jan 1 00:00:00 0001 (now=Jan 1 00:00:00 2019))`,
		},
		{
			name: "organization",
			cert: &x509.Certificate{
				Subject: pkix.Name{
					CommonName:   "test-subject",
					Organization: []string{"foo", "bar"},
				},
				Issuer: pkix.Name{
					CommonName: "test-issuer",
				},
			},
			expected: `"test-subject" [] groups=[foo,bar] issuer="test-issuer" (Jan 1 00:00:00 0001 to Jan 1 00:00:00 0001 (now=Jan 1 00:00:00 2019))`,
		},
		{
			name: "client auth",
			cert: &x509.Certificate{
				Subject: pkix.Name{
					CommonName: "test-subject",
				},
				Issuer: pkix.Name{
					CommonName: "test-issuer",
				},
				ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			},
			expected: `"test-subject" [client] issuer="test-issuer" (Jan 1 00:00:00 0001 to Jan 1 00:00:00 0001 (now=Jan 1 00:00:00 2019))`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := CertificateToString(test.cert)
			if out != test.expected {
				t.Errorf("expected %q, got %q", test.expected, out)
			}
		})
	}
}
