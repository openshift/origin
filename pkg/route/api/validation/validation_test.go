package validation

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/intstr"

	"github.com/openshift/origin/pkg/route/api"
)

const (
	testExpiredCAUnknownCertificate = `-----BEGIN CERTIFICATE-----
MIIDIjCCAgqgAwIBAgIBBjANBgkqhkiG9w0BAQUFADCBoTELMAkGA1UEBhMCVVMx
CzAJBgNVBAgMAlNDMRUwEwYDVQQHDAxEZWZhdWx0IENpdHkxHDAaBgNVBAoME0Rl
ZmF1bHQgQ29tcGFueSBMdGQxEDAOBgNVBAsMB1Rlc3QgQ0ExGjAYBgNVBAMMEXd3
dy5leGFtcGxlY2EuY29tMSIwIAYJKoZIhvcNAQkBFhNleGFtcGxlQGV4YW1wbGUu
Y29tMB4XDTE2MDExMzE5NDA1N1oXDTI2MDExMDE5NDA1N1owfDEYMBYGA1UEAxMP
d3d3LmV4YW1wbGUuY29tMQswCQYDVQQIEwJTQzELMAkGA1UEBhMCVVMxIjAgBgkq
hkiG9w0BCQEWE2V4YW1wbGVAZXhhbXBsZS5jb20xEDAOBgNVBAoTB0V4YW1wbGUx
EDAOBgNVBAsTB0V4YW1wbGUwgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGBAM0B
u++oHV1wcphWRbMLUft8fD7nPG95xs7UeLPphFZuShIhhdAQMpvcsFeg+Bg9PWCu
v3jZljmk06MLvuWLfwjYfo9q/V+qOZVfTVHHbaIO5RTXJMC2Nn+ACF0kHBmNcbth
OOgF8L854a/P8tjm1iPR++vHnkex0NH7lyosVc/vAgMBAAGjDTALMAkGA1UdEwQC
MAAwDQYJKoZIhvcNAQEFBQADggEBADjFm5AlNH3DNT1Uzx3m66fFjqqrHEs25geT
yA3rvBuynflEHQO95M/8wCxYVyuAx4Z1i4YDC7tx0vmOn/2GXZHY9MAj1I8KCnwt
Jik7E2r1/yY0MrkawljOAxisXs821kJ+Z/51Ud2t5uhGxS6hJypbGspMS7OtBbw7
8oThK7cWtCXOldNF6ruqY1agWnhRdAq5qSMnuBXuicOP0Kbtx51a1ugE3SnvQenJ
nZxdtYUXvEsHZC/6bAtTfNh+/SwgxQJuL2ZM+VG3X2JIKY8xTDui+il7uTh422lq
wED8uwKl+bOj6xFDyw4gWoBxRobsbFaME8pkykP1+GnKDberyAM=
-----END CERTIFICATE-----`

	testExpiredCertPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIICWwIBAAKBgQDNAbvvqB1dcHKYVkWzC1H7fHw+5zxvecbO1Hiz6YRWbkoSIYXQ
EDKb3LBXoPgYPT1grr942ZY5pNOjC77li38I2H6Pav1fqjmVX01Rx22iDuUU1yTA
tjZ/gAhdJBwZjXG7YTjoBfC/OeGvz/LY5tYj0fvrx55HsdDR+5cqLFXP7wIDAQAB
AoGAfE7P4Zsj6zOzGPI/Izj7Bi5OvGnEeKfzyBiH9Dflue74VRQkqqwXs/DWsNv3
c+M2Y3iyu5ncgKmUduo5X8D9To2ymPRLGuCdfZTxnBMpIDKSJ0FTwVPkr6cYyyBk
5VCbc470pQPxTAAtl2eaO1sIrzR4PcgwqrSOjwBQQocsGAECQQD8QOra/mZmxPbt
bRh8U5lhgZmirImk5RY3QMPI/1/f4k+fyjkU5FRq/yqSyin75aSAXg8IupAFRgyZ
W7BT6zwBAkEA0A0ugAGorpCbuTa25SsIOMxkEzCiKYvh0O+GfGkzWG4lkSeJqGME
keuJGlXrZNKNoCYLluAKLPmnd72X2yTL7wJARM0kAXUP0wn324w8+HQIyqqBj/gF
Vt9Q7uMQQ3s72CGu3ANZDFS2nbRZFU5koxrggk6lRRk1fOq9NvrmHg10AQJABOea
pgfj+yGLmkUw8JwgGH6xCUbHO+WBUFSlPf+Y50fJeO+OrjqPXAVKeSV3ZCwWjKT4
9viXJNJJ4WfF0bO/XwJAOMB1wQnEOSZ4v+laMwNtMq6hre5K8woqteXICoGcIWe8
u3YLAbyW/lHhOCiZu2iAI8AbmXem9lW6Tr7p/97s0w==
-----END RSA PRIVATE KEY-----`

	testCertificate = `-----BEGIN CERTIFICATE-----
MIICwjCCAiugAwIBAgIBATANBgkqhkiG9w0BAQsFADBjMQswCQYDVQQGEwJVUzEL
MAkGA1UECAwCQ0ExETAPBgNVBAoMCFNlY3VyaXR5MRswGQYDVQQLDBJPcGVuU2hp
ZnQzIHRlc3QgQ0ExFzAVBgNVBAMMDmhlYWRlci50ZXN0IENBMB4XDTE2MDMxMjA0
MjEwM1oXDTM2MDMxMjA0MjEwM1owWDEUMBIGA1UEAwwLaGVhZGVyLnRlc3QxCzAJ
BgNVBAgMAkNBMQswCQYDVQQGEwJVUzERMA8GA1UECgwIU2VjdXJpdHkxEzARBgNV
BAsMCk9wZW5TaGlmdDMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQD0
XEAzUMflZy8zluwzqMKnu8jYK3yUoEGLN0Bw0A/7ydno1g0E92ee8M9p59TCCWA6
nKnt1DEK5285xAKs9AveutSYiDkpf2px59GvCVx2ecfFBTECWHMAJ/6Y7pqlWOt2
hvPx5rP+jVeNLAfK9d+f57FGvWXrQAcBnFTegS6J910kbvDgNP4Nerj6RPAx2UOq
6URqA4j7qZs63nReeu/1t//BQHNokKddfxw2ZXcL/5itgpPug16thp+ugGVdjcFs
aasLJOjErUS0D+7bot98FL0TSpxWqwtCF117bSLY7UczZFNAZAOnZBFmSZBxcJJa
TZzkda0Oiqo0J3GPcZ+rAgMBAAGjDTALMAkGA1UdEwQCMAAwDQYJKoZIhvcNAQEL
BQADgYEACkdKRUm9ERjgbe6w0fw4VY1s5XC9qR1m5AwLMVVwKxHJVG2zMzeDTHyg
3cjxmfZdFU9yxmNUCh3mRsi2+qjEoFfGRyMwMMx7cduYhsFY3KA+Fl4vBRXAuPLR
eCI4ErCPi+Y08vOto9VVXg2f4YFQYLq1X6TiXD5RpQAN0t8AYk4=
-----END CERTIFICATE-----`

	testPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA9FxAM1DH5WcvM5bsM6jCp7vI2Ct8lKBBizdAcNAP+8nZ6NYN
BPdnnvDPaefUwglgOpyp7dQxCudvOcQCrPQL3rrUmIg5KX9qcefRrwlcdnnHxQUx
AlhzACf+mO6apVjrdobz8eaz/o1XjSwHyvXfn+exRr1l60AHAZxU3oEuifddJG7w
4DT+DXq4+kTwMdlDqulEagOI+6mbOt50Xnrv9bf/wUBzaJCnXX8cNmV3C/+YrYKT
7oNerYafroBlXY3BbGmrCyToxK1EtA/u26LffBS9E0qcVqsLQhdde20i2O1HM2RT
QGQDp2QRZkmQcXCSWk2c5HWtDoqqNCdxj3GfqwIDAQABAoIBAEfl+NHge+CIur+w
MXGFvziBLThFm1NTz9U5fZFz9q/8FUzH5m7GqMuASVb86oHpJlI4lFsw6vktXXGe
tbbT28Y+LJ1wv3jxT42SSwT4eSc278uNmnz5L2UlX2j6E7CA+E8YqCBN5DoKtm8I
PIbAT3sKPgP1aE6OuUEFEYeidOIMvjco2aQH0338sl6cObkQFEgnWf2ncun3KGnb
s+dMO5EdYLo0rOdDXY88sElfqiNYYl/FRu9O3OfqHvScA5uo9FlIhukcrRkbjFcq
j/7k4tt0iLs9B2j+4ihBWYo5eRFIde4Izj6a6ArEk0ShEUvwlZBuGMM/vs+jvbDK
l3+0NpECgYEA/+qxwvOGjmlYNKFK/rzxd51EnfCISnV+tb17pNyRmlGToi1/LmmV
+jcJfcwlf2o8mTFn3xAdD3fSaHF7t8Li7xDwH2S+sSuFE/8bhgHUvw1S7oILMYyO
hO6sWG+JocMhr8IejaAnQxav9VvP01YDfw/XBB0O1EIuzzr2KHq+AGMCgYEA9HCY
JGTcv7lfs3kcCAkDtjl8NbjNRMxRErG0dfYS+6OSaXOOMg1TsaSNEgjOGyUX+yQ4
4vtKcLwHk7+qz3ZPbhS6m7theZG9jUwMrQRGyCE7z3JUy8vmV/N+HP0V+boT+4KM
Tai3+I3hf9+QMHYx/Z/VA0K6f27LwP+kEL9C8hkCgYEAoiHeXNRL+w1ihHVrPdgW
YuGQBz/MGOA3VoylON1Eoa/tCGIqoQzjp5IWwUwEtaRon+VdGUTsJFCVTPYYm2Ms
wqjIeBsrdLNNrE2C8nNWhXO7hr98t/eEk1NifOStHX6yaNdi4/cC6M4GzDtOf2WO
8YDniAOg0Xjcjw2bxil9FmECgYBuUeq4cjUW6okArSYzki30rhka/d7WsAffEgjK
PFbw7zADG74PZOhjAksQ2px6r9EU7ZInDxbXrmUVD6n9m/3ZRs25v2YMwfP0s1/9
LjLr2+PsikMu/0VkaGaAmtCyNoMSPicoXX86VH5zgejHlnCVcO9oW1NkdBLNdhML
4+ZI8QKBgQDb+SH7i50Yu3adwvPkDSp3ACCzPoHXno79a7Y5S2JzpFtNq+cNLWEb
HP8gHJSZnaGrLKmjwNeQNsARYajKmDKO5HJ9g5H5Hae8enOb2yie541dneDT8rID
4054dMQJnijd8620yf8wiNy05ZPOQQ0JvA/rW3WWZc5PGm8c2PsVjg==
-----END RSA PRIVATE KEY-----`

	testCACertificate = `-----BEGIN CERTIFICATE-----
MIIClDCCAf2gAwIBAgIJAPU57OGhuqJtMA0GCSqGSIb3DQEBCwUAMGMxCzAJBgNV
BAYTAlVTMQswCQYDVQQIDAJDQTERMA8GA1UECgwIU2VjdXJpdHkxGzAZBgNVBAsM
Ek9wZW5TaGlmdDMgdGVzdCBDQTEXMBUGA1UEAwwOaGVhZGVyLnRlc3QgQ0EwHhcN
MTYwMzEyMDQyMTAzWhcNMzYwMzEyMDQyMTAzWjBjMQswCQYDVQQGEwJVUzELMAkG
A1UECAwCQ0ExETAPBgNVBAoMCFNlY3VyaXR5MRswGQYDVQQLDBJPcGVuU2hpZnQz
IHRlc3QgQ0ExFzAVBgNVBAMMDmhlYWRlci50ZXN0IENBMIGfMA0GCSqGSIb3DQEB
AQUAA4GNADCBiQKBgQCsdVIJ6GSrkFdE9LzsMItYGE4q3qqSqIbs/uwMoVsMT+33
pLeyzeecPuoQsdO6SEuqhUM1ivUN4GyXIR1+aW2baMwMXpjX9VIJu5d4FqtGi6SD
RfV+tbERWwifPJlN+ryuvqbbDxrjQeXhemeo7yrJdgJ1oyDmoM5pTiSUUmltvQID
AQABo1AwTjAdBgNVHQ4EFgQUOVuieqGfp2wnKo7lX2fQt+Yk1C4wHwYDVR0jBBgw
FoAUOVuieqGfp2wnKo7lX2fQt+Yk1C4wDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0B
AQsFAAOBgQA8VhmNeicRnKgXInVyYZDjL0P4WRbKJY7DkJxRMRWxikbEVHdySki6
jegpqgJqYbzU6EiuTS2sl2bAjIK9nGUtTDt1PJIC1Evn5Q6v5ylNflpv6GxtUbCt
bGvtpjWA4r9WASIDPFsxk/cDEEEO6iPxgMOf5MdpQC2y2MU0rzF/Gg==
-----END CERTIFICATE-----`

	testDestinationCACertificate = testCACertificate
)

func createRouteSpecTo(name string, kind string) api.RouteTargetReference {
	svc := api.RouteTargetReference{
		Name: name,
		Kind: kind,
	}
	return svc
}

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
				Spec: api.RouteSpec{
					Host: "host",
					To:   createRouteSpecTo("serviceName", "Service"),
				},
			},
			expectedErrors: 1,
		},
		{
			name: "No namespace",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name: "name",
				},
				Spec: api.RouteSpec{
					Host: "host",
					To:   createRouteSpecTo("serviceName", "Service"),
				},
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
				Spec: api.RouteSpec{
					To: createRouteSpecTo("serviceName", "Service"),
				},
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
				Spec: api.RouteSpec{
					Host: "**",
					To:   createRouteSpecTo("serviceName", "Service"),
				},
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
				Spec: api.RouteSpec{
					Host: "host",
					To:   createRouteSpecTo("", "Service"),
				},
			},
			expectedErrors: 1,
		},
		{
			name: "No service kind",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: api.RouteSpec{
					Host: "host",
					To:   createRouteSpecTo("serviceName", ""),
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Zero port",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: api.RouteSpec{
					Host: "www.example.com",
					To:   createRouteSpecTo("serviceName", "Service"),
					Port: &api.RoutePort{
						TargetPort: intstr.FromInt(0),
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Empty string port",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: api.RouteSpec{
					Host: "www.example.com",
					To:   createRouteSpecTo("serviceName", "Service"),
					Port: &api.RoutePort{
						TargetPort: intstr.FromString(""),
					},
				},
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
				Spec: api.RouteSpec{
					Host: "www.example.com",
					To:   createRouteSpecTo("serviceName", "Service"),
				},
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
				Spec: api.RouteSpec{
					Host: "www.example.com",
					To:   createRouteSpecTo("serviceName", "Service"),
					Path: "/test",
				},
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
				Spec: api.RouteSpec{
					Host: "www.example.com",
					To:   createRouteSpecTo("serviceName", "Service"),
					Path: "test",
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough route with path",
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: api.RouteSpec{
					Host: "www.example.com",
					Path: "/test",
					To:   createRouteSpecTo("serviceName", "Service"),
					TLS: &api.TLSConfig{
						Termination: api.TLSTerminationPassthrough,
					},
				},
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

func TestValidateTLS(t *testing.T) {
	tests := []struct {
		name           string
		route          *api.Route
		expectedErrors int
	}{
		{
			name: "No TLS Termination",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination: "",
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination OK",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination: api.TLSTerminationPassthrough,
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Reencrypt termination OK with certs",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination:              api.TLSTerminationReencrypt,
						Certificate:              "def",
						Key:                      "ghi",
						CACertificate:            "jkl",
						DestinationCACertificate: "abc",
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Reencrypt termination OK without certs",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination:              api.TLSTerminationReencrypt,
						DestinationCACertificate: "abc",
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Reencrypt termination no dest cert",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination:   api.TLSTerminationReencrypt,
						Certificate:   "def",
						Key:           "ghi",
						CACertificate: "jkl",
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Edge termination OK with certs",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination:   api.TLSTerminationEdge,
						Certificate:   "abc",
						Key:           "abc",
						CACertificate: "abc",
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Edge termination OK without certs",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination: api.TLSTerminationEdge,
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Edge termination, dest cert",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination:              api.TLSTerminationEdge,
						DestinationCACertificate: "abc",
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination, cert",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{Termination: api.TLSTerminationPassthrough, Certificate: "test"},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination, key",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{Termination: api.TLSTerminationPassthrough, Key: "test"},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination, ca cert",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{Termination: api.TLSTerminationPassthrough, CACertificate: "test"},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination, dest ca cert",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{Termination: api.TLSTerminationPassthrough, DestinationCACertificate: "test"},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Invalid termination type",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination: "invalid",
					},
				},
			},
			expectedErrors: 1,
		},
	}

	for _, tc := range tests {
		errs := validateTLS(tc.route, nil)

		if len(errs) != tc.expectedErrors {
			t.Errorf("Test case %s expected %d error(s), got %d. %v", tc.name, tc.expectedErrors, len(errs), errs)
		}
	}
}

func TestValidateTLSInsecureEdgeTerminationPolicy(t *testing.T) {
	tests := []struct {
		name  string
		route *api.Route
	}{
		{
			name: "Passthrough termination",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination: api.TLSTerminationPassthrough,
					},
				},
			},
		},
		{
			name: "Reencrypt termination",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination:              api.TLSTerminationReencrypt,
						DestinationCACertificate: "dca",
					},
				},
			},
		},
		{
			name: "Reencrypt termination DestCACert",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination:              api.TLSTerminationReencrypt,
						DestinationCACertificate: testDestinationCACertificate,
					},
				},
			},
		},
	}

	insecureTypes := []api.InsecureEdgeTerminationPolicyType{
		api.InsecureEdgeTerminationPolicyNone,
		api.InsecureEdgeTerminationPolicyAllow,
		api.InsecureEdgeTerminationPolicyRedirect,
		"support HTTPsec",
		"or maybe HSTS",
	}

	for _, tc := range tests {
		if errs := validateTLS(tc.route, nil); len(errs) != 0 {
			t.Errorf("Test case %s got %d errors where none were expected. %v",
				tc.name, len(errs), errs)
		}

		tc.route.Spec.TLS.InsecureEdgeTerminationPolicy = ""
		if errs := validateTLS(tc.route, nil); len(errs) != 0 {
			t.Errorf("Test case %s got %d errors where none were expected. %v",
				tc.name, len(errs), errs)
		}

		for _, val := range insecureTypes {
			tc.route.Spec.TLS.InsecureEdgeTerminationPolicy = val
			if errs := validateTLS(tc.route, nil); len(errs) != 1 {
				t.Errorf("Test case %s with insecure=%q got %d errors where one was expected. %v",
					tc.name, val, len(errs), errs)
			}
		}
	}
}

// TestValidateRouteBad ensures not specifying a required field results in error and a fully specified
// route passes successfully
func TestValidateRouteUpdate(t *testing.T) {
	tests := []struct {
		name           string
		route          *api.Route
		change         func(route *api.Route)
		expectedErrors int
	}{
		{
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:            "bar",
					Namespace:       "foo",
					ResourceVersion: "1",
				},
				Spec: api.RouteSpec{
					Host: "host",
					To: api.RouteTargetReference{
						Name: "serviceName",
						Kind: "Service",
					},
				},
			},
			change:         func(route *api.Route) { route.Spec.Host = "" },
			expectedErrors: 1,
		},
		{
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:            "bar",
					Namespace:       "foo",
					ResourceVersion: "1",
				},
				Spec: api.RouteSpec{
					Host: "host",
					To: api.RouteTargetReference{
						Name: "serviceName",
						Kind: "Service",
					},
				},
			},
			change:         func(route *api.Route) { route.Spec.Host = "other" },
			expectedErrors: 1,
		},
		{
			route: &api.Route{
				ObjectMeta: kapi.ObjectMeta{
					Name:            "bar",
					Namespace:       "foo",
					ResourceVersion: "1",
				},
				Spec: api.RouteSpec{
					Host: "host",
					To: api.RouteTargetReference{
						Name: "serviceName",
						Kind: "Service",
					},
				},
			},
			change:         func(route *api.Route) { route.Name = "baz" },
			expectedErrors: 1,
		},
	}

	for i, tc := range tests {
		copied, err := kapi.Scheme.Copy(tc.route)
		if err != nil {
			t.Fatal(err)
		}
		newRoute := copied.(*api.Route)
		tc.change(newRoute)
		errs := ValidateRouteUpdate(newRoute, tc.route)
		if len(errs) != tc.expectedErrors {
			t.Errorf("%d: expected %d error(s), got %d. %v", i, tc.expectedErrors, len(errs), errs)
		}
	}
}

func TestValidateInsecureEdgeTerminationPolicy(t *testing.T) {
	tests := []struct {
		name           string
		insecure       api.InsecureEdgeTerminationPolicyType
		expectedErrors int
	}{
		{
			name:           "empty insecure option",
			insecure:       "",
			expectedErrors: 0,
		},
		{
			name:           "foobar insecure option",
			insecure:       "foobar",
			expectedErrors: 1,
		},
		{
			name:           "insecure option none",
			insecure:       api.InsecureEdgeTerminationPolicyNone,
			expectedErrors: 0,
		},
		{
			name:           "insecure option allow",
			insecure:       api.InsecureEdgeTerminationPolicyAllow,
			expectedErrors: 0,
		},
		{
			name:           "insecure option redirect",
			insecure:       api.InsecureEdgeTerminationPolicyRedirect,
			expectedErrors: 0,
		},
		{
			name:           "insecure option other",
			insecure:       "something else",
			expectedErrors: 1,
		},
	}

	for _, tc := range tests {
		route := &api.Route{
			Spec: api.RouteSpec{
				TLS: &api.TLSConfig{
					Termination:                   api.TLSTerminationEdge,
					InsecureEdgeTerminationPolicy: tc.insecure,
				},
			},
		}
		errs := validateTLS(route, nil)

		if len(errs) != tc.expectedErrors {
			t.Errorf("Test case %s expected %d error(s), got %d. %v", tc.name, tc.expectedErrors, len(errs), errs)
		}
	}
}

func TestValidateNoTLSInsecureEdgeTerminationPolicy(t *testing.T) {
	insecureTypes := map[api.InsecureEdgeTerminationPolicyType]bool{
		api.InsecureEdgeTerminationPolicyNone:     false,
		api.InsecureEdgeTerminationPolicyAllow:    false,
		api.InsecureEdgeTerminationPolicyRedirect: false,
		"support HTTPsec":                         true,
		"or maybe HSTS":                           true,
	}

	for key, expected := range insecureTypes {
		route := &api.Route{
			Spec: api.RouteSpec{
				TLS: &api.TLSConfig{
					Termination:                   api.TLSTerminationEdge,
					InsecureEdgeTerminationPolicy: key,
				},
			},
		}
		errs := validateTLS(route, nil)
		if !expected && len(errs) != 0 {
			t.Errorf("Test case for edge termination with insecure=%s got %d errors where none were expected. %v",
				key, len(errs), errs)
		}
		if expected && len(errs) == 0 {
			t.Errorf("Test case for edge termination with insecure=%s got no errors where some were expected.", key)
		}
	}
}

// TestExtendedValidateRoute ensures that a route's certificate and keys
// are valid.
func TestExtendedValidateRoute(t *testing.T) {
	tests := []struct {
		name           string
		route          *api.Route
		expectedErrors int
	}{
		{
			name: "No TLS Termination",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination: "",
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination OK",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination: api.TLSTerminationPassthrough,
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Reencrypt termination OK with certs",
			route: &api.Route{
				Spec: api.RouteSpec{
					Host: "www.example.com",

					TLS: &api.TLSConfig{
						Termination:              api.TLSTerminationReencrypt,
						Certificate:              testCertificate,
						Key:                      testPrivateKey,
						CACertificate:            testCACertificate,
						DestinationCACertificate: testDestinationCACertificate,
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Reencrypt termination OK with bad config",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination:              api.TLSTerminationReencrypt,
						Certificate:              "def",
						Key:                      "ghi",
						CACertificate:            "jkl",
						DestinationCACertificate: "abc",
					},
				},
			},
			expectedErrors: 4,
		},
		{
			name: "Reencrypt termination OK without certs",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination:              api.TLSTerminationReencrypt,
						DestinationCACertificate: testDestinationCACertificate,
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Reencrypt termination bad config without certs",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination:              api.TLSTerminationReencrypt,
						DestinationCACertificate: "abc",
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Reencrypt termination no dest cert",
			route: &api.Route{
				Spec: api.RouteSpec{
					Host: "www.example.com",
					TLS: &api.TLSConfig{
						Termination:   api.TLSTerminationReencrypt,
						Certificate:   testCertificate,
						Key:           testPrivateKey,
						CACertificate: testCACertificate,
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Edge termination OK with certs without host",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination:   api.TLSTerminationEdge,
						Certificate:   testCertificate,
						Key:           testPrivateKey,
						CACertificate: testCACertificate,
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Edge termination OK with certs",
			route: &api.Route{
				Spec: api.RouteSpec{
					Host: "www.example.com",
					TLS: &api.TLSConfig{
						Termination:   api.TLSTerminationEdge,
						Certificate:   testCertificate,
						Key:           testPrivateKey,
						CACertificate: testCACertificate,
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Edge termination bad config with certs",
			route: &api.Route{
				Spec: api.RouteSpec{
					Host: "www.example.com",
					TLS: &api.TLSConfig{
						Termination:   api.TLSTerminationEdge,
						Certificate:   "abc",
						Key:           "abc",
						CACertificate: "abc",
					},
				},
			},
			expectedErrors: 3,
		},
		{
			name: "Edge termination mismatched key and cert",
			route: &api.Route{
				Spec: api.RouteSpec{
					Host: "www.example.com",
					TLS: &api.TLSConfig{
						Termination:   api.TLSTerminationEdge,
						Certificate:   testCertificate,
						Key:           testExpiredCertPrivateKey,
						CACertificate: testCACertificate,
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Edge termination expired cert",
			route: &api.Route{
				Spec: api.RouteSpec{
					Host: "www.example.com",
					TLS: &api.TLSConfig{
						Termination:   api.TLSTerminationEdge,
						Certificate:   testExpiredCAUnknownCertificate,
						Key:           testExpiredCertPrivateKey,
						CACertificate: testCACertificate,
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Edge termination expired cert key mismatch",
			route: &api.Route{
				Spec: api.RouteSpec{
					Host: "www.example.com",
					TLS: &api.TLSConfig{
						Termination:   api.TLSTerminationEdge,
						Certificate:   testExpiredCAUnknownCertificate,
						Key:           testPrivateKey,
						CACertificate: testCACertificate,
					},
				},
			},
			expectedErrors: 2,
		},
		{
			name: "Edge termination OK without certs",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination: api.TLSTerminationEdge,
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Edge termination, dest cert",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination:              api.TLSTerminationEdge,
						DestinationCACertificate: "abc",
					},
				},
			},
			expectedErrors: 2,
		},
		{
			name: "Passthrough termination, cert",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{Termination: api.TLSTerminationPassthrough, Certificate: "test"},
				},
			},
			expectedErrors: 3,
		},
		{
			name: "Passthrough termination, key",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{Termination: api.TLSTerminationPassthrough, Key: "test"},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination, ca cert",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{Termination: api.TLSTerminationPassthrough, CACertificate: "test"},
				},
			},
			expectedErrors: 2,
		},
		{
			name: "Passthrough termination, dest ca cert",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{Termination: api.TLSTerminationPassthrough, DestinationCACertificate: "test"},
				},
			},
			expectedErrors: 2,
		},
		{
			name: "Invalid termination type",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination: "invalid",
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Double escaped newlines",
			route: &api.Route{
				Spec: api.RouteSpec{
					TLS: &api.TLSConfig{
						Termination:              api.TLSTerminationReencrypt,
						Certificate:              "d\\nef",
						Key:                      "g\\nhi",
						CACertificate:            "j\\nkl",
						DestinationCACertificate: "j\\nkl",
					},
				},
			},
			expectedErrors: 4,
		},
	}

	for _, tc := range tests {
		errs := ExtendedValidateRoute(tc.route)

		if len(errs) != tc.expectedErrors {
			t.Errorf("Test case %s expected %d error(s), got %d. %v", tc.name, tc.expectedErrors, len(errs), errs)
		}
	}
}
