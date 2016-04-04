package validation

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/intstr"

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
				Spec: api.RouteSpec{
					Host: "host",
					To: kapi.ObjectReference{
						Name: "serviceName",
						Kind: "Service",
					},
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
					To: kapi.ObjectReference{
						Name: "serviceName",
						Kind: "Service",
					},
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
					To: kapi.ObjectReference{
						Name: "serviceName",
						Kind: "Service",
					},
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
					To: kapi.ObjectReference{
						Name: "serviceName",
						Kind: "Service",
					},
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
					To: kapi.ObjectReference{
						Kind: "Service",
					},
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
					To: kapi.ObjectReference{
						Name: "serviceName",
					},
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
					To: kapi.ObjectReference{
						Name: "serviceName",
						Kind: "Service",
					},
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
					To: kapi.ObjectReference{
						Name: "serviceName",
						Kind: "Service",
					},
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
					To: kapi.ObjectReference{
						Name: "serviceName",
						Kind: "Service",
					},
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
					To: kapi.ObjectReference{
						Name: "serviceName",
						Kind: "Service",
					},
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
					To: kapi.ObjectReference{
						Name: "serviceName",
						Kind: "Service",
					},
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
					To: kapi.ObjectReference{
						Name: "serviceName",
						Kind: "Service",
					},
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
