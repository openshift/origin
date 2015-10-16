package validation

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/cmd/server/api"
)

func TestValidateServingInfo(t *testing.T) {
	certFile, err := ioutil.TempFile("", "cert.crt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(certFile.Name())
	certFileName := certFile.Name()
	ioutil.WriteFile(certFile.Name(), localhostCert, os.FileMode(0755))

	keyFile, err := ioutil.TempFile("", "cert.key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(keyFile.Name())
	keyFileName := keyFile.Name()
	ioutil.WriteFile(keyFile.Name(), localhostKey, os.FileMode(0755))

	testcases := map[string]struct {
		ServingInfo      api.ServingInfo
		ExpectedErrors   []string
		ExpectedWarnings []string
	}{
		"basic": {
			ServingInfo: api.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
			},
		},
		"missing key": {
			ServingInfo: api.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert: api.CertInfo{
					CertFile: certFileName,
				},
			},
			ExpectedErrors: []string{"keyFile: required"},
		},

		"namedCertificates valid": {
			ServingInfo: api.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  api.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []api.NamedCertificate{
					{Names: []string{"example.com"}, CertInfo: api.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
		},

		"namedCertificates without default cert": {
			ServingInfo: api.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				//ServerCert:  api.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []api.NamedCertificate{
					{Names: []string{"example.com"}, CertInfo: api.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
			ExpectedErrors: []string{"namedCertificates: invalid"},
		},

		"namedCertificates with missing names": {
			ServingInfo: api.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  api.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []api.NamedCertificate{
					{Names: []string{ /*"example.com"*/ }, CertInfo: api.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
			ExpectedErrors: []string{"namedCertificates[0].names: required"},
		},
		"namedCertificates with missing key": {
			ServingInfo: api.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  api.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []api.NamedCertificate{
					{Names: []string{"example.com"}, CertInfo: api.CertInfo{CertFile: certFileName /*, KeyFile: keyFileName*/}},
				},
			},
			ExpectedErrors: []string{"namedCertificates[0].keyFile: required"},
		},
		"namedCertificates with duplicate names": {
			ServingInfo: api.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  api.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []api.NamedCertificate{
					{Names: []string{"example.com"}, CertInfo: api.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
					{Names: []string{"example.com"}, CertInfo: api.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
			ExpectedErrors: []string{"namedCertificates[1].names[0]: invalid"},
		},
		"namedCertificates with empty name": {
			ServingInfo: api.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  api.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []api.NamedCertificate{
					{Names: []string{""}, CertInfo: api.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
			ExpectedErrors: []string{"namedCertificates[0].names[0]: required"},
		},

		"namedCertificates with unmatched DNS name": {
			ServingInfo: api.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  api.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []api.NamedCertificate{
					{Names: []string{"badexample.com"}, CertInfo: api.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
			ExpectedWarnings: []string{"namedCertificates[0].names[0]: invalid"},
		},
		"namedCertificates with non-DNS names": {
			ServingInfo: api.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  api.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []api.NamedCertificate{
					{Names: []string{"foo bar.com"}, CertInfo: api.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
			ExpectedErrors: []string{
				"namedCertificates[0].names[0]: invalid value 'foo bar.com', Details: must be a valid DNS name",
			},
		},
	}

	for k, tc := range testcases {
		result := ValidateServingInfo(tc.ServingInfo)

		if len(tc.ExpectedErrors) != len(result.Errors) {
			t.Errorf("%s: Expected %d errors, got %d", k, len(tc.ExpectedErrors), len(result.Errors))
			for _, e := range tc.ExpectedErrors {
				t.Logf("\tExpected error: %s", e)
			}
			for _, r := range result.Errors {
				t.Logf("\tActual error: %s", r.Error())
			}
			continue
		}
		for i, r := range result.Errors {
			if !strings.Contains(r.Error(), tc.ExpectedErrors[i]) {
				t.Errorf("%s: Expected error containing %s, got %s", k, tc.ExpectedErrors[i], r.Error())
			}
		}

		if len(tc.ExpectedWarnings) != len(result.Warnings) {
			t.Errorf("%s: Expected %d warning, got %d", k, len(tc.ExpectedWarnings), len(result.Warnings))
			for _, e := range tc.ExpectedErrors {
				t.Logf("\tExpected warning: %s", e)
			}
			for _, r := range result.Warnings {
				t.Logf("\tActual warning: %s", r.Error())
			}
			continue
		}
		for i, r := range result.Warnings {
			if !strings.Contains(r.Error(), tc.ExpectedWarnings[i]) {
				t.Errorf("%s: Expected warning containing %s, got %s", k, tc.ExpectedWarnings[i], r.Error())
			}
		}
	}
}

// localhostCert is a PEM-encoded TLS cert with SAN IPs
// "127.0.0.1" and "[::1]", expiring at the last second of 2049 (the end
// of ASN.1 time).
// generated from src/crypto/tls:
// go run generate_cert.go  --rsa-bits 512 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIBdzCCASOgAwIBAgIBADALBgkqhkiG9w0BAQUwEjEQMA4GA1UEChMHQWNtZSBD
bzAeFw03MDAxMDEwMDAwMDBaFw00OTEyMzEyMzU5NTlaMBIxEDAOBgNVBAoTB0Fj
bWUgQ28wWjALBgkqhkiG9w0BAQEDSwAwSAJBAN55NcYKZeInyTuhcCwFMhDHCmwa
IUSdtXdcbItRB/yfXGBhiex00IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEA
AaNoMGYwDgYDVR0PAQH/BAQDAgCkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1Ud
EwEB/wQFMAMBAf8wLgYDVR0RBCcwJYILZXhhbXBsZS5jb22HBH8AAAGHEAAAAAAA
AAAAAAAAAAAAAAEwCwYJKoZIhvcNAQEFA0EAAoQn/ytgqpiLcZu9XKbCJsJcvkgk
Se6AbGXgSlq+ZCEVo0qIwSgeBqmsJxUu7NCSOwVJLYNEBO2DtIxoYVk+MA==
-----END CERTIFICATE-----`)

// localhostKey is the private key for localhostCert.
var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBPAIBAAJBAN55NcYKZeInyTuhcCwFMhDHCmwaIUSdtXdcbItRB/yfXGBhiex0
0IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEAAQJBAQdUx66rfh8sYsgfdcvV
NoafYpnEcB5s4m/vSVe6SU7dCK6eYec9f9wpT353ljhDUHq3EbmE4foNzJngh35d
AekCIQDhRQG5Li0Wj8TM4obOnnXUXf1jRv0UkzE9AHWLG5q3AwIhAPzSjpYUDjVW
MCUXgckTpKCuGwbJk7424Nb8bLzf3kllAiA5mUBgjfr/WtFSJdWcPQ4Zt9KTMNKD
EUO0ukpTwEIl6wIhAMbGqZK3zAAFdq8DD2jPx+UJXnh0rnOkZBzDtJ6/iN69AiEA
1Aq8MJgTaYsDQWyU/hDq5YkDJc9e9DSCvUIzqxQWMQE=
-----END RSA PRIVATE KEY-----`)
