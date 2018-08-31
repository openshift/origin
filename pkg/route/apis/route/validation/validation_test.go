package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

const (
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

func createRouteSpecTo(name string, kind string) routeapi.RouteTargetReference {
	svc := routeapi.RouteTargetReference{
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
		route          *routeapi.Route
		expectedErrors int
	}{
		{
			name: "No Name",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host: "host",
					To:   createRouteSpecTo("serviceName", "Service"),
				},
			},
			expectedErrors: 1,
		},
		{
			name: "No namespace",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name",
				},
				Spec: routeapi.RouteSpec{
					Host: "host",
					To:   createRouteSpecTo("serviceName", "Service"),
				},
			},
			expectedErrors: 1,
		},
		{
			name: "No host",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					To: createRouteSpecTo("serviceName", "Service"),
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Invalid DNS 952 host",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host: "**",
					To:   createRouteSpecTo("serviceName", "Service"),
				},
			},
			expectedErrors: 1,
		},
		{
			name: "No service name",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host: "host",
					To:   createRouteSpecTo("", "Service"),
				},
			},
			expectedErrors: 1,
		},
		{
			name: "No service kind",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host: "host",
					To:   createRouteSpecTo("serviceName", ""),
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Zero port",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					To:   createRouteSpecTo("serviceName", "Service"),
					Port: &routeapi.RoutePort{
						TargetPort: intstr.FromInt(0),
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Empty string port",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					To:   createRouteSpecTo("serviceName", "Service"),
					Port: &routeapi.RoutePort{
						TargetPort: intstr.FromString(""),
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Valid route",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					To:   createRouteSpecTo("serviceName", "Service"),
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Valid route with path",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					To:   createRouteSpecTo("serviceName", "Service"),
					Path: "/test",
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Invalid route with path",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					To:   createRouteSpecTo("serviceName", "Service"),
					Path: "test",
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough route with path",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					Path: "/test",
					To:   createRouteSpecTo("serviceName", "Service"),
					TLS: &routeapi.TLSConfig{
						Termination: routeapi.TLSTerminationPassthrough,
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "No wildcard policy",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nowildcard",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host: "no.wildcard.test",
					To:   createRouteSpecTo("serviceName", "Service"),
				},
			},
			expectedErrors: 0,
		},
		{
			name: "wildcard policy none",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nowildcard2",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host:           "none.wildcard.test",
					To:             createRouteSpecTo("serviceName", "Service"),
					WildcardPolicy: routeapi.WildcardPolicyNone,
				},
			},
			expectedErrors: 0,
		},
		{
			name: "wildcard policy subdomain",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wildcardpolicy",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host:           "subdomain.wildcard.test",
					To:             createRouteSpecTo("serviceName", "Service"),
					WildcardPolicy: routeapi.WildcardPolicySubdomain,
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Invalid wildcard policy",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "badwildcard",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host:           "bad.wildcard.test",
					To:             createRouteSpecTo("serviceName", "Service"),
					WildcardPolicy: "bad-wolf",
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Invalid host for wildcard policy",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "badhost",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					To:             createRouteSpecTo("serviceName", "Service"),
					WildcardPolicy: routeapi.WildcardPolicySubdomain,
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Empty host for wildcard policy",
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "emptyhost",
					Namespace: "foo",
				},
				Spec: routeapi.RouteSpec{
					Host:           "",
					To:             createRouteSpecTo("serviceName", "Service"),
					WildcardPolicy: routeapi.WildcardPolicySubdomain,
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
		route          *routeapi.Route
		expectedErrors int
	}{
		{
			name: "No TLS Termination",
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{
						Termination: "",
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination OK",
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{
						Termination: routeapi.TLSTerminationPassthrough,
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Reencrypt termination OK with certs",
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{
						Termination:              routeapi.TLSTerminationReencrypt,
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
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{
						Termination:              routeapi.TLSTerminationReencrypt,
						DestinationCACertificate: "abc",
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Reencrypt termination no dest cert",
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{
						Termination:   routeapi.TLSTerminationReencrypt,
						Certificate:   "def",
						Key:           "ghi",
						CACertificate: "jkl",
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Edge termination OK with certs",
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{
						Termination:   routeapi.TLSTerminationEdge,
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
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{
						Termination: routeapi.TLSTerminationEdge,
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Edge termination, dest cert",
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{
						Termination:              routeapi.TLSTerminationEdge,
						DestinationCACertificate: "abc",
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination, cert",
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{Termination: routeapi.TLSTerminationPassthrough, Certificate: "test"},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination, key",
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{Termination: routeapi.TLSTerminationPassthrough, Key: "test"},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination, ca cert",
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{Termination: routeapi.TLSTerminationPassthrough, CACertificate: "test"},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination, dest ca cert",
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{Termination: routeapi.TLSTerminationPassthrough, DestinationCACertificate: "test"},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Invalid termination type",
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{
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

func TestValidatePassthroughInsecureEdgeTerminationPolicy(t *testing.T) {

	insecureTypes := map[routeapi.InsecureEdgeTerminationPolicyType]bool{
		"": false,
		routeapi.InsecureEdgeTerminationPolicyNone:     false,
		routeapi.InsecureEdgeTerminationPolicyAllow:    true,
		routeapi.InsecureEdgeTerminationPolicyRedirect: false,
		"support HTTPsec":                              true,
		"or maybe HSTS":                                true,
	}

	for key, expected := range insecureTypes {
		route := &routeapi.Route{
			Spec: routeapi.RouteSpec{
				TLS: &routeapi.TLSConfig{
					Termination:                   routeapi.TLSTerminationPassthrough,
					InsecureEdgeTerminationPolicy: key,
				},
			},
		}
		route.Spec.TLS.InsecureEdgeTerminationPolicy = key
		errs := validateTLS(route, nil)
		if !expected && len(errs) != 0 {
			t.Errorf("Test case for Passthrough termination with insecure=%s got %d errors where none where expected. %v",
				key, len(errs), errs)
		}
		if expected && len(errs) == 0 {
			t.Errorf("Test case for Passthrough termination with insecure=%s got no errors where some where expected.", key)
		}
	}
}

// TestValidateRouteBad ensures not specifying a required field results in error and a fully specified
// route passes successfully
func TestValidateRouteUpdate(t *testing.T) {
	tests := []struct {
		name           string
		route          *routeapi.Route
		change         func(route *routeapi.Route)
		expectedErrors int
	}{
		{
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "bar",
					Namespace:       "foo",
					ResourceVersion: "1",
				},
				Spec: routeapi.RouteSpec{
					Host: "host",
					To: routeapi.RouteTargetReference{
						Name: "serviceName",
						Kind: "Service",
					},
				},
			},
			change:         func(route *routeapi.Route) { route.Spec.Host = "" },
			expectedErrors: 0, // now controlled by rbac
		},
		{
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "bar",
					Namespace:       "foo",
					ResourceVersion: "1",
				},
				Spec: routeapi.RouteSpec{
					Host: "host",
					To: routeapi.RouteTargetReference{
						Name: "serviceName",
						Kind: "Service",
					},
				},
			},
			change:         func(route *routeapi.Route) { route.Spec.Host = "other" },
			expectedErrors: 0, // now controlled by rbac
		},
		{
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "bar",
					Namespace:       "foo",
					ResourceVersion: "1",
				},
				Spec: routeapi.RouteSpec{
					Host: "host",
					To: routeapi.RouteTargetReference{
						Name: "serviceName",
						Kind: "Service",
					},
				},
			},
			change:         func(route *routeapi.Route) { route.Name = "baz" },
			expectedErrors: 1,
		},
	}

	for i, tc := range tests {
		newRoute := tc.route.DeepCopy()
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
		insecure       routeapi.InsecureEdgeTerminationPolicyType
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
			insecure:       routeapi.InsecureEdgeTerminationPolicyNone,
			expectedErrors: 0,
		},
		{
			name:           "insecure option allow",
			insecure:       routeapi.InsecureEdgeTerminationPolicyAllow,
			expectedErrors: 0,
		},
		{
			name:           "insecure option redirect",
			insecure:       routeapi.InsecureEdgeTerminationPolicyRedirect,
			expectedErrors: 0,
		},
		{
			name:           "insecure option other",
			insecure:       "something else",
			expectedErrors: 1,
		},
	}

	for _, tc := range tests {
		route := &routeapi.Route{
			Spec: routeapi.RouteSpec{
				TLS: &routeapi.TLSConfig{
					Termination:                   routeapi.TLSTerminationEdge,
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

func TestValidateEdgeReencryptInsecureEdgeTerminationPolicy(t *testing.T) {
	tests := []struct {
		name  string
		route *routeapi.Route
	}{
		{
			name: "Reencrypt termination",
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{
						Termination:              routeapi.TLSTerminationReencrypt,
						DestinationCACertificate: "dca",
					},
				},
			},
		},
		{
			name: "Reencrypt termination DestCACert",
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{
						Termination:              routeapi.TLSTerminationReencrypt,
						DestinationCACertificate: testDestinationCACertificate,
					},
				},
			},
		},
		{
			name: "Edge termination",
			route: &routeapi.Route{
				Spec: routeapi.RouteSpec{
					TLS: &routeapi.TLSConfig{
						Termination: routeapi.TLSTerminationEdge,
					},
				},
			},
		},
	}

	insecureTypes := map[routeapi.InsecureEdgeTerminationPolicyType]bool{
		routeapi.InsecureEdgeTerminationPolicyNone:     false,
		routeapi.InsecureEdgeTerminationPolicyAllow:    false,
		routeapi.InsecureEdgeTerminationPolicyRedirect: false,
		"support HTTPsec":                              true,
		"or maybe HSTS":                                true,
	}

	for _, tc := range tests {
		for key, expected := range insecureTypes {
			tc.route.Spec.TLS.InsecureEdgeTerminationPolicy = key
			errs := validateTLS(tc.route, nil)
			if !expected && len(errs) != 0 {
				t.Errorf("Test case %s with insecure=%s got %d errors where none were expected. %v",
					tc.name, key, len(errs), errs)
			}
			if expected && len(errs) == 0 {
				t.Errorf("Test case %s  with insecure=%s got no errors where some were expected.", tc.name, key)
			}
		}
	}
}
