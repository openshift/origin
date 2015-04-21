package validation

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/route/api"
)

// TestValidateRouteBad ensures not specifying a required field results in error and a fully specified
// route passes successfully
func TestValidateRoute(t *testing.T) {
	tests := []struct {
		name           string
		route          *api.Route
		expectedErrors int
	}{
		{
			name: "No Name",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
				},
				Host:        "host",
				ServiceName: "serviceName",
			},
			expectedErrors: 1,
		},
		{
			name: "No namespace",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name: "name",
				},
				Host:        "host",
				ServiceName: "serviceName",
			},
			expectedErrors: 1,
		},
		{
			name: "No host",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				ServiceName: "serviceName",
			},
			expectedErrors: 0,
		},
		{
			name: "Invalid DNS 952 host",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Host:        "**",
				ServiceName: "serviceName",
			},
			expectedErrors: 1,
		},
		{
			name: "No service name",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Host: "host",
			},
			expectedErrors: 1,
		},
		{
			name: "Valid route",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Host:        "www.example.com",
				ServiceName: "serviceName",
			},
			expectedErrors: 0,
		},
		{
			name: "Valid route with path",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Host:        "www.example.com",
				Path:        "/test",
				ServiceName: "serviceName",
			},
			expectedErrors: 0,
		},
		{
			name: "Invalid route with path",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Host:        "www.example.com",
				Path:        "test",
				ServiceName: "serviceName",
			},
			expectedErrors: 1,
		},
	}

	for _, tc := range tests {
		errs := ValidateRoute(tc.route)

		if len(errs) != tc.expectedErrors {
			t.Errorf("Test case %s expected %d error(s), got %d. %v", tc.name, tc.expectedErrors, len(errs), errs)
		}
	}
}

func TestValidateTLSNoTLSTermOk(t *testing.T) {
	errs := validateTLS(&api.TLSConfig{
		Termination: "",
	})

	if len(errs) > 0 {
		t.Errorf("Unexpected non-empty error list: %#v", errs)
	}
}

func TestValidateTLSPodTermoOK(t *testing.T) {
	errs := validateTLS(&api.TLSConfig{
		Termination: api.TLSTerminationPassthrough,
	})

	if len(errs) > 0 {
		t.Errorf("Unexpected non-empty error list: %#v", errs)
	}
}

func TestValidateTLSReencryptTermOKCert(t *testing.T) {
	errs := validateTLS(&api.TLSConfig{
		Termination:              api.TLSTerminationReencrypt,
		DestinationCACertificate: validCACert,
		Certificate:              validCert,
		Key:                      validKey,
		CACertificate:            validCACert,
	})

	if len(errs) > 0 {
		t.Errorf("Unexpected non-empty error list: %#v", errs)
	}
}

func TestValidateTLSEdgeTermOKCerts(t *testing.T) {
	errs := validateTLS(&api.TLSConfig{
		Termination:   api.TLSTerminationEdge,
		Certificate:   validCert,
		Key:           validKey,
		CACertificate: validCACert,
	})

	if len(errs) > 0 {
		t.Errorf("Unexpected non-empty error list: %#v", errs)
	}
}

func TestValidateEdgeTermInvalid(t *testing.T) {
	testCases := []struct {
		name string
		cfg  api.TLSConfig
	}{
		{"no cert", api.TLSConfig{
			Termination:   api.TLSTerminationEdge,
			Key:           "abc",
			CACertificate: "abc",
		}},
		{"no key", api.TLSConfig{
			Termination:   api.TLSTerminationEdge,
			Certificate:   "abc",
			CACertificate: "abc",
		}},
		{"no ca cert", api.TLSConfig{
			Termination: api.TLSTerminationEdge,
			Certificate: "abc",
			Key:         "abc",
		}},
	}

	for _, tc := range testCases {
		errs := validateTLS(&tc.cfg)

		if len(errs) != 1 {
			t.Errorf("Unexpected error list encountered for case %v: %#v.  Expected 1 error, got %v", tc.name, errs, len(errs))
		}
	}
}

func TestValidatePodTermInvalid(t *testing.T) {
	testCases := []struct {
		name string
		cfg  api.TLSConfig
	}{
		{"cert", api.TLSConfig{Termination: api.TLSTerminationPassthrough, Certificate: "test"}},
		{"key", api.TLSConfig{Termination: api.TLSTerminationPassthrough, Key: "test"}},
		{"ca cert", api.TLSConfig{Termination: api.TLSTerminationPassthrough, CACertificate: "test"}},
		{"dest cert", api.TLSConfig{Termination: api.TLSTerminationPassthrough, DestinationCACertificate: "test"}},
	}

	for _, tc := range testCases {
		errs := validateTLS(&tc.cfg)

		if len(errs) != 1 {
			t.Errorf("Unexpected error list encountered for test case %v: %#v.  Expected 1 error, got %v", tc.name, errs, len(errs))
		}
	}
}

// TestValidateReencryptTermInvalid ensures reencrypt must specify cert, key, cacert, and dest cert
func TestValidateReencryptTermInvalid(t *testing.T) {
	errs := validateTLS(&api.TLSConfig{
		Termination: api.TLSTerminationReencrypt,
	})

	if len(errs) != 4 {
		t.Errorf("Unexpected error list encountered: %#v.  Expected 4 errors, got %v", errs, len(errs))
	}
}

func TestValidateTLSInvalidTermination(t *testing.T) {
	errs := validateTLS(&api.TLSConfig{
		Termination: "invalid",
	})

	if len(errs) != 1 {
		t.Errorf("Unexpected error list encountered: %#v.  Expected 1 errors, got %v", errs, len(errs))
	}
}

func TestValidateTLSCertificates(t *testing.T) {
	testCases := []struct {
		name         string
		cert         string
		key          string
		cacert       string
		destcert     string
		expectedErrs int
	}{
		{
			name:         "valid cert/key, valid cacert, no dest cert",
			cert:         validCert,
			key:          validKey,
			cacert:       validCACert,
			expectedErrs: 0,
		},
		{
			name:         "valid cert/key, valid cacert, valid dest cert",
			cert:         validCert,
			key:          validKey,
			cacert:       validCACert,
			destcert:     validCACert,
			expectedErrs: 0,
		},
		{
			name:         "valid cert/key, valid cacert, invalid dest cert",
			cert:         validCert,
			key:          validKey,
			cacert:       validCACert,
			destcert:     invalidCACert,
			expectedErrs: 1,
		},
		{
			name:         "valid cert/key, valid chained cacert, valid chained dest cert",
			cert:         validCert,
			key:          validKey,
			cacert:       validChain,
			destcert:     validChain,
			expectedErrs: 0,
		},
		{
			name:         "valid cert/key, invalid chained cacert, valid dest cert",
			cert:         validCert,
			key:          validKey,
			cacert:       invalidChain,
			destcert:     validCACert,
			expectedErrs: 1,
		},
		{
			name:         "valid cert/key, valid chained cacert, invalid dest cert",
			cert:         validCert,
			key:          validKey,
			cacert:       validCACert,
			destcert:     invalidChain,
			expectedErrs: 1,
		},
		{
			name:         "valid cert/key without ca cert",
			cert:         validCert,
			key:          validKey,
			expectedErrs: 0,
		},
		{
			name:         "invalid cert, valid ca, valid key",
			cert:         invalidCert,
			key:          validKey,
			cacert:       validCACert,
			expectedErrs: 1,
		},
		{
			name:         "valid cert/key, invalid ca, valid key",
			cert:         validCert,
			key:          validKey,
			cacert:       invalidCACert,
			expectedErrs: 1,
		},
		{
			name:         "valid cert, valid ca, invalid key",
			cert:         validCert,
			key:          invalidKey,
			cacert:       validCACert,
			expectedErrs: 1,
		},
	}

	for _, tc := range testCases {
		tls := &api.TLSConfig{
			Certificate:              tc.cert,
			Key:                      tc.key,
			CACertificate:            tc.cacert,
			DestinationCACertificate: tc.destcert,
		}

		errs := validateTLSCertificates(tls)
		if len(errs) != tc.expectedErrs {
			t.Errorf("test case %s failed, expected %d error(s) but got %d: %+v", tc.name, tc.expectedErrs, len(errs), errs)
		}
	}
}

// These certificates are example certificates generated by a fake cert authority.
// In order to regenerate these certificates (or create new ones) you will need to grab the demo CA key
// which can be found https://github.com/pweil-/hello-nginx-docker.  That repo contains all the keys found below, the
// CA configuration file used to sign the keys, and the CA keys themselves along with the CA database.
//
// The CA certificate/key was generated with:
// OPENSSL=ca.cnf openssl req -x509 -nodes -days 3650 -newkey rsa:2048 -out mypersonalca/certs/ca.pem -outform PEM -keyout ./mypersonalca/private/ca.key
//
// In order to create new certificates you must first make a certificate request and key using openssl.  You will be asked
// a series of questions.  The important one is the Common Name.  The certificates below marked Example* use www.example.com
// as the common name.  Example2* uses www.example2.com as the common name
//
// openssl req -newkey rsa:1024 -nodes -sha1 -keyout cert.key -keyform PEM -out cert.req -outform PEM
//
// Once you have the request you then need to generate the the certificate with the authority key
// OPENSSL_CONF=ca.cnf openssl ca -batch -notext -in cert.req -out cert.pem
//
// To view your certificate via the command line:
// openssl x509 -in cert.pem -noout -text
const (
	validCert = `-----BEGIN CERTIFICATE-----
MIIDGjCCAgKgAwIBAgIBBDANBgkqhkiG9w0BAQUFADCBoTELMAkGA1UEBhMCVVMx
CzAJBgNVBAgMAlNDMRUwEwYDVQQHDAxEZWZhdWx0IENpdHkxHDAaBgNVBAoME0Rl
ZmF1bHQgQ29tcGFueSBMdGQxEDAOBgNVBAsMB1Rlc3QgQ0ExGjAYBgNVBAMMEXd3
dy5leGFtcGxlY2EuY29tMSIwIAYJKoZIhvcNAQkBFhNleGFtcGxlQGV4YW1wbGUu
Y29tMB4XDTE1MDQxNjIwNDYyNFoXDTE2MDQxNTIwNDYyNFowdDEfMB0GA1UEAwwW
Ki5yb3V0ZXIuZGVmYXVsdC5sb2NhbDELMAkGA1UECAwCU0MxCzAJBgNVBAYTAlVT
MSIwIAYJKoZIhvcNAQkBFhNleGFtcGxlQGV4YW1wbGUuY29tMRMwEQYDVQQKDApE
TyBOT1QgVVNFMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDJvNTsletojsSW
R6jrKLVgJ6VTJy9Q/ehQfPDQmdVLdEuVQHoqPPZi5RM/sowjjDKXb38mWj9oiNwu
jjdjpT8AJU/NgWbwml8G8bybyuxUN/LaTMsxQVO+CRoCIfTkZyfifRcnsAUyZU0M
nb0ibFKKaAF7HA27JSuz2xw2jRt9tQIDAQABow0wCzAJBgNVHRMEAjAAMA0GCSqG
SIb3DQEBBQUAA4IBAQBuWhysctwWNgk9EhgOux0Txnhmo5lC9ACF9RQWqoG8KC37
N0rTPL4q/rkn9Cw776hp2Bq2GRlkT7EssNDrdH0tTPf8W+jDjAU7joF4f8iXC6Hh
zLEgek2diOrmNBSpMowuHr1+5JmHsyyq2jeJ+g2MfO+lOaYmrzAp7P2WlgJo7IMk
vZulWJvbqm1slRKosZc4TQiih2SThBrKvloJRbHJcPmj6XbD2511R4PEORsO0ELw
I0IINEhcFvPzNZluHOUT+/3iWTJBu/zFGuWYLLDxPAjOrkVw/Go/H7AJPBCGB7pA
fO51fmV+dGFRx4fp+SUS3yMnQNwwiaKNMVd5xqGB
-----END CERTIFICATE-----`
	validCACert = `-----BEGIN CERTIFICATE-----
MIIEFzCCAv+gAwIBAgIJALK1iUpF2VQLMA0GCSqGSIb3DQEBBQUAMIGhMQswCQYD
VQQGEwJVUzELMAkGA1UECAwCU0MxFTATBgNVBAcMDERlZmF1bHQgQ2l0eTEcMBoG
A1UECgwTRGVmYXVsdCBDb21wYW55IEx0ZDEQMA4GA1UECwwHVGVzdCBDQTEaMBgG
A1UEAwwRd3d3LmV4YW1wbGVjYS5jb20xIjAgBgkqhkiG9w0BCQEWE2V4YW1wbGVA
ZXhhbXBsZS5jb20wHhcNMTUwMTEyMTQxNTAxWhcNMjUwMTA5MTQxNTAxWjCBoTEL
MAkGA1UEBhMCVVMxCzAJBgNVBAgMAlNDMRUwEwYDVQQHDAxEZWZhdWx0IENpdHkx
HDAaBgNVBAoME0RlZmF1bHQgQ29tcGFueSBMdGQxEDAOBgNVBAsMB1Rlc3QgQ0Ex
GjAYBgNVBAMMEXd3dy5leGFtcGxlY2EuY29tMSIwIAYJKoZIhvcNAQkBFhNleGFt
cGxlQGV4YW1wbGUuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
w2rK1J2NMtQj0KDug7g7HRKl5jbf0QMkMKyTU1fBtZ0cCzvsF4CqV11LK4BSVWaK
rzkaXe99IVJnH8KdOlDl5Dh/+cJ3xdkClSyeUT4zgb6CCBqg78ePp+nN11JKuJlV
IG1qdJpB1J5O/kCLsGcTf7RS74MtqMFo96446Zvt7YaBhWPz6gDaO/TUzfrNcGLA
EfHVXkvVWqb3gqXUztZyVex/gtP9FXQ7gxTvJml7UkmT0VAFjtZnCqmFxpLZFZ15
+qP9O7Q2MpsGUO/4vDAuYrKBeg1ZdPSi8gwqUP2qWsGd9MIWRv3thI2903BczDc7
r8WaIbm37vYZAS9G56E4+wIDAQABo1AwTjAdBgNVHQ4EFgQUugLrSJshOBk5TSsU
ANs4+SmJUGwwHwYDVR0jBBgwFoAUugLrSJshOBk5TSsUANs4+SmJUGwwDAYDVR0T
BAUwAwEB/zANBgkqhkiG9w0BAQUFAAOCAQEAaMJ33zAMV4korHo5aPfayV3uHoYZ
1ChzP3eSsF+FjoscpoNSKs91ZXZF6LquzoNezbfiihK4PYqgwVD2+O0/Ty7UjN4S
qzFKVR4OS/6lCJ8YncxoFpTntbvjgojf1DEataKFUN196PAANc3yz8cWHF4uvjPv
WkgFqbIjb+7D1YgglNyovXkRDlRZl0LD1OQ0ZWhd4Ge1qx8mmmanoBeYZ9+DgpFC
j9tQAbS867yeOryNe7sEOIpXAAqK/DTu0hB6+ySsDfMo4piXCc2aA/eI2DCuw08e
w17Dz9WnupZjVdwTKzDhFgJZMLDqn37HQnT6EemLFqbcR0VPEnfyhDtZIQ==
-----END CERTIFICATE-----`
	validKey = `-----BEGIN PRIVATE KEY-----
MIICeAIBADANBgkqhkiG9w0BAQEFAASCAmIwggJeAgEAAoGBAMm81OyV62iOxJZH
qOsotWAnpVMnL1D96FB88NCZ1Ut0S5VAeio89mLlEz+yjCOMMpdvfyZaP2iI3C6O
N2OlPwAlT82BZvCaXwbxvJvK7FQ38tpMyzFBU74JGgIh9ORnJ+J9FyewBTJlTQyd
vSJsUopoAXscDbslK7PbHDaNG321AgMBAAECgYEArTYy44e9fiLG6/lPMcncIVko
/AJy/+liJGmCIrlSh9ysYNPhkI6TRkpFgrV82bCwZ5HV7Eokk06fLmHxcN8a/TC1
8QPQTpsNeLcKZa5reNfp0Hh/Fqw+/gFpLci+qn30kjevNHCSCalQKAyVQw7TOUek
pshRA4c6ojbVdoWm6ikCQQDoJ8TwN/UMHqzQtAf9S1v8+J6WG+r6lmM4kvWS9cKQ
1wOYFFgrzPxqhOR2pluM1RaJZ/JbJCVNolDslZ9mrHMDAkEA3nVEVJQIuY2YLDYK
rYNiEpWGH6u4eL14zZCiSrc8RqgAdaauzCwUss9IxOn+Rr57i424xguSHDlkBFZl
YfiS5wJAWe0dwhdK2pj/RBCYj6sjRMhhVbAWw16BrKZwba644TYIdF5dEQpkNDap
8LPb/p+EDVGwdVF5Cat4QUxr5G+kVQJBAMgI5Le1IZ9Qjpx6v+FEugSCBcgmzstr
fNxECVtsJzxVx4wDpTydCsO7FvFSg76zfD6B4rvbHbhZdvFbivCs59MCQQCSC7k1
Y+QydYPU19kUIS0kj23kkl3dg1jKs7OHnsUKMGqKM0uLclC2pk4nlXenkNjdfCae
pYTptRezoVsjKraW
-----END PRIVATE KEY-----`
	invalidCert = `-----BEGIN CERTIFICATE-----
invalidCAgKgAwIBAgIBBDANBgkqhkiG9w0BAQUFADCBoTELMAkGA1UEBhMCVVMx
CzAJBgNVBAgMAlNDMRUwEwYDVQQHDAxEZWZhdWx0IENpdHkxHDAaBgNVBAoME0Rl
ZmF1bHQgQ29tcGFueSBMdGQxEDAOBgNVBAsMB1Rlc3QgQ0ExGjAYBgNVBAMMEXd3
dy5leGFtcGxlY2EuY29tMSIwIAYJKoZIhvcNAQkBFhNleGFtcGxlQGV4YW1wbGUu
Y29tMB4XDTE1MDQxNjIwNDYyNFoXDTE2MDQxNTIwNDYyNFowdDEfMB0GA1UEAwwW
Ki5yb3V0ZXIuZGVmYXVsdC5sb2NhbDELMAkGA1UECAwCU0MxCzAJBgNVBAYTAlVT
MSIwIAYJKoZIhvcNAQkBFhNleGFtcGxlQGV4YW1wbGUuY29tMRMwEQYDVQQKDApE
TyBOT1QgVVNFMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDJvNTsletojsSW
R6jrKLVgJ6VTJy9Q/ehQfPDQmdVLdEuVQHoqPPZi5RM/sowjjDKXb38mWj9oiNwu
jjdjpT8AJU/NgWbwml8G8bybyuxUN/LaTMsxQVO+CRoCIfTkZyfifRcnsAUyZU0M
nb0ibFKKaAF7HA27JSuz2xw2jRt9tQIDAQABow0wCzAJBgNVHRMEAjAAMA0GCSqG
SIb3DQEBBQUAA4IBAQBuWhysctwWNgk9EhgOux0Txnhmo5lC9ACF9RQWqoG8KC37
N0rTPL4q/rkn9Cw776hp2Bq2GRlkT7EssNDrdH0tTPf8W+jDjAU7joF4f8iXC6Hh
zLEgek2diOrmNBSpMowuHr1+5JmHsyyq2jeJ+g2MfO+lOaYmrzAp7P2WlgJo7IMk
vZulWJvbqm1slRKosZc4TQiih2SThBrKvloJRbHJcPmj6XbD2511R4PEORsO0ELw
I0IINEhcFvPzNZluHOUT+/3iWTJBu/zFGuWYLLDxPAjOrkVw/Go/H7AJPBCGB7pA
fO51fmV+dGFRx4fp+SUS3yMnQNwwiaKNMVd5xqGB
-----END CERTIFICATE-----`
	invalidCACert = `-----BEGIN CERTIFICATE-----
invalidCAv+gAwIBAgIJALK1iUpF2VQLMA0GCSqGSIb3DQEBBQUAMIGhMQswCQYD
VQQGEwJVUzELMAkGA1UECAwCU0MxFTATBgNVBAcMDERlZmF1bHQgQ2l0eTEcMBoG
A1UECgwTRGVmYXVsdCBDb21wYW55IEx0ZDEQMA4GA1UECwwHVGVzdCBDQTEaMBgG
A1UEAwwRd3d3LmV4YW1wbGVjYS5jb20xIjAgBgkqhkiG9w0BCQEWE2V4YW1wbGVA
ZXhhbXBsZS5jb20wHhcNMTUwMTEyMTQxNTAxWhcNMjUwMTA5MTQxNTAxWjCBoTEL
MAkGA1UEBhMCVVMxCzAJBgNVBAgMAlNDMRUwEwYDVQQHDAxEZWZhdWx0IENpdHkx
HDAaBgNVBAoME0RlZmF1bHQgQ29tcGFueSBMdGQxEDAOBgNVBAsMB1Rlc3QgQ0Ex
GjAYBgNVBAMMEXd3dy5leGFtcGxlY2EuY29tMSIwIAYJKoZIhvcNAQkBFhNleGFt
cGxlQGV4YW1wbGUuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
w2rK1J2NMtQj0KDug7g7HRKl5jbf0QMkMKyTU1fBtZ0cCzvsF4CqV11LK4BSVWaK
rzkaXe99IVJnH8KdOlDl5Dh/+cJ3xdkClSyeUT4zgb6CCBqg78ePp+nN11JKuJlV
IG1qdJpB1J5O/kCLsGcTf7RS74MtqMFo96446Zvt7YaBhWPz6gDaO/TUzfrNcGLA
EfHVXkvVWqb3gqXUztZyVex/gtP9FXQ7gxTvJml7UkmT0VAFjtZnCqmFxpLZFZ15
+qP9O7Q2MpsGUO/4vDAuYrKBeg1ZdPSi8gwqUP2qWsGd9MIWRv3thI2903BczDc7
r8WaIbm37vYZAS9G56E4+wIDAQABo1AwTjAdBgNVHQ4EFgQUugLrSJshOBk5TSsU
ANs4+SmJUGwwHwYDVR0jBBgwFoAUugLrSJshOBk5TSsUANs4+SmJUGwwDAYDVR0T
BAUwAwEB/zANBgkqhkiG9w0BAQUFAAOCAQEAaMJ33zAMV4korHo5aPfayV3uHoYZ
1ChzP3eSsF+FjoscpoNSKs91ZXZF6LquzoNezbfiihK4PYqgwVD2+O0/Ty7UjN4S
qzFKVR4OS/6lCJ8YncxoFpTntbvjgojf1DEataKFUN196PAANc3yz8cWHF4uvjPv
WkgFqbIjb+7D1YgglNyovXkRDlRZl0LD1OQ0ZWhd4Ge1qx8mmmanoBeYZ9+DgpFC
j9tQAbS867yeOryNe7sEOIpXAAqK/DTu0hB6+ySsDfMo4piXCc2aA/eI2DCuw08e
w17Dz9WnupZjVdwTKzDhFgJZMLDqn37HQnT6EemLFqbcR0VPEnfyhDtZIQ==
-----END CERTIFICATE-----`
	invalidKey = `-----BEGIN PRIVATE KEY-----
invalidBADANBgkqhkiG9w0BAQEFAASCAmIwggJeAgEAAoGBAMm81OyV62iOxJZH
qOsotWAnpVMnL1D96FB88NCZ1Ut0S5VAeio89mLlEz+yjCOMMpdvfyZaP2iI3C6O
N2OlPwAlT82BZvCaXwbxvJvK7FQ38tpMyzFBU74JGgIh9ORnJ+J9FyewBTJlTQyd
vSJsUopoAXscDbslK7PbHDaNG321AgMBAAECgYEArTYy44e9fiLG6/lPMcncIVko
/AJy/+liJGmCIrlSh9ysYNPhkI6TRkpFgrV82bCwZ5HV7Eokk06fLmHxcN8a/TC1
8QPQTpsNeLcKZa5reNfp0Hh/Fqw+/gFpLci+qn30kjevNHCSCalQKAyVQw7TOUek
pshRA4c6ojbVdoWm6ikCQQDoJ8TwN/UMHqzQtAf9S1v8+J6WG+r6lmM4kvWS9cKQ
1wOYFFgrzPxqhOR2pluM1RaJZ/JbJCVNolDslZ9mrHMDAkEA3nVEVJQIuY2YLDYK
rYNiEpWGH6u4eL14zZCiSrc8RqgAdaauzCwUss9IxOn+Rr57i424xguSHDlkBFZl
YfiS5wJAWe0dwhdK2pj/RBCYj6sjRMhhVbAWw16BrKZwba644TYIdF5dEQpkNDap
8LPb/p+EDVGwdVF5Cat4QUxr5G+kVQJBAMgI5Le1IZ9Qjpx6v+FEugSCBcgmzstr
fNxECVtsJzxVx4wDpTydCsO7FvFSg76zfD6B4rvbHbhZdvFbivCs59MCQQCSC7k1
Y+QydYPU19kUIS0kj23kkl3dg1jKs7OHnsUKMGqKM0uLclC2pk4nlXenkNjdfCae
pYTptRezoVsjKraW
-----END PRIVATE KEY-----`
	validChain   = validCACert + "\n" + validCACert + "\n" + validCACert
	invalidChain = validCACert + "\n" + invalidCACert + "\n" + validCACert
)
