package validation

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/cmd/server/apis/config"
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
		ServingInfo      config.ServingInfo
		ExpectedErrors   []string
		ExpectedWarnings []string
	}{
		"basic": {
			ServingInfo: config.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
			},
			ExpectedErrors: []string{
				"certFile: Required value: The certificate file must be provided",
				"keyFile: Required value: The certificate key must be provided",
			},
		},
		"missing key": {
			ServingInfo: config.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert: config.CertInfo{
					CertFile: certFileName,
				},
			},
			ExpectedErrors: []string{
				"keyFile: Required value: The certificate key must be provided",
				"certFile: Required value: Both the certificate file and the certificate key must be provided together or not at all",
				"keyFile: Required value: Both the certificate file and the certificate key must be provided together or not at all",
			},
		},

		"namedCertificates valid": {
			ServingInfo: config.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  config.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []config.NamedCertificate{
					{Names: []string{"example.com"}, CertInfo: config.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
		},
		"namedCertificates valid wildcard spec": {
			ServingInfo: config.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  config.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []config.NamedCertificate{
					{Names: []string{"*.wildcard.com"}, CertInfo: config.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
		},
		"namedCertificates specific host for wildcard cert": {
			ServingInfo: config.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  config.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []config.NamedCertificate{
					{Names: []string{"www.wildcard.com"}, CertInfo: config.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
		},

		"namedCertificates without default cert": {
			ServingInfo: config.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				//ServerCert:  api.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []config.NamedCertificate{
					{Names: []string{"example.com"}, CertInfo: config.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
			ExpectedErrors: []string{
				"certFile: Required value: The certificate file must be provided",
				"keyFile: Required value: The certificate key must be provided",
				"namedCertificates: Invalid value",
			},
		},

		"namedCertificates with missing names": {
			ServingInfo: config.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  config.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []config.NamedCertificate{
					{Names: []string{ /*"example.com"*/ }, CertInfo: config.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
			ExpectedErrors: []string{"namedCertificates[0].names: Required value"},
		},
		"namedCertificates with missing key": {
			ServingInfo: config.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  config.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []config.NamedCertificate{
					{Names: []string{"example.com"}, CertInfo: config.CertInfo{CertFile: certFileName /*, KeyFile: keyFileName*/}},
				},
			},
			ExpectedErrors: []string{
				"namedCertificates[0].certFile: Required value: Both the certificate file and the certificate key must be provided together or not at all",
				"namedCertificates[0].keyFile: Required value: Both the certificate file and the certificate key must be provided together or not at all",
			},
		},
		"namedCertificates with duplicate names": {
			ServingInfo: config.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  config.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []config.NamedCertificate{
					{Names: []string{"example.com"}, CertInfo: config.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
					{Names: []string{"example.com"}, CertInfo: config.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
			ExpectedErrors: []string{"namedCertificates[1].names[0]: Invalid value"},
		},
		"namedCertificates with empty name": {
			ServingInfo: config.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  config.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []config.NamedCertificate{
					{Names: []string{""}, CertInfo: config.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
			ExpectedErrors: []string{"namedCertificates[0].names[0]: Required value"},
		},

		"namedCertificates with unmatched DNS name": {
			ServingInfo: config.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  config.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []config.NamedCertificate{
					{Names: []string{"badexample.com"}, CertInfo: config.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
			ExpectedWarnings: []string{"namedCertificates[0].names[0]: Invalid value"},
		},
		"namedCertificates with non-DNS names": {
			ServingInfo: config.ServingInfo{
				BindAddress: "0.0.0.0:1234",
				BindNetwork: "tcp",
				ServerCert:  config.CertInfo{CertFile: certFileName, KeyFile: keyFileName},
				NamedCertificates: []config.NamedCertificate{
					{Names: []string{"foo bar.com"}, CertInfo: config.CertInfo{CertFile: certFileName, KeyFile: keyFileName}},
				},
			},
			ExpectedErrors: []string{
				`namedCertificates[0].names[0]: Invalid value: "foo bar.com": must be a valid DNS name`,
			},
		},
	}

	for k, tc := range testcases {
		result := ValidateServingInfo(tc.ServingInfo, true, nil)

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
// go run generate_cert.go  --rsa-bits 512 --host 127.0.0.1,::1,example.com,*.wildcard.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIBmjCCAUagAwIBAgIQNElZIQ+5sNqQ5FlhpXDzvzALBgkqhkiG9w0BAQswEjEQ
MA4GA1UEChMHQWNtZSBDbzAgFw03MDAxMDEwMDAwMDBaGA8yMDg0MDEyOTE2MDAw
MFowEjEQMA4GA1UEChMHQWNtZSBDbzBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQDX
oyZQ4OZGzWC+UqL+F671Gtv6wxyrQWbyu8z5KxrHCxObGTMG4fcSOTrJ5ApwIXuW
O6KuXL/QwbdI+0V43pNhAgMBAAGjeDB2MA4GA1UdDwEB/wQEAwIApDATBgNVHSUE
DDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MD4GA1UdEQQ3MDWCC2V4YW1w
bGUuY29tgg4qLndpbGRjYXJkLmNvbYcEfwAAAYcQAAAAAAAAAAAAAAAAAAAAATAL
BgkqhkiG9w0BAQsDQQDHWUY1n4YZNm2Cuutg5NGaRefzzK9qgksi7bIs9bH0tYPH
/Vp4NKH+27aG54X5U+Vw1aXS9CKhqEky5CZMfHtn
-----END CERTIFICATE-----`)

// localhostKey is the private key for localhostCert.
var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBOwIBAAJBANejJlDg5kbNYL5Sov4XrvUa2/rDHKtBZvK7zPkrGscLE5sZMwbh
9xI5OsnkCnAhe5Y7oq5cv9DBt0j7RXjek2ECAwEAAQJBAIMFMma5/7DNYRbDBx30
Le3nX/nBS04S8wZRbX2H30FIL/PU4mezFiDoVlcIEHUBi1TAcwQux3FFg/8f+j6w
rAECIQDzWRsqow24qQL5nPCvA9RSkNgmZSCpog5hKSK1vgNS8QIhAOLZOJlLVo8v
IUaAt4uvQJVE/ClFi7sLq2hnduJjiGdxAiBCcldHqiQqAwRL8j2KHGqSbPiIa16i
0xxIDXpr08mGkQIgfV1CVCU4buTC5O2Zgc6WSGfZWw2eDP6D+azEHJSY+2ECIQCU
+w6O+Pa96Fi0XvY8wVsg1h1eNUjAumxThaf9Sp64lw==
-----END RSA PRIVATE KEY-----`)
