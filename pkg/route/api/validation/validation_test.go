package validation

import (
	"testing"

	"github.com/openshift/origin/pkg/route/api"
	kapi "k8s.io/kubernetes/pkg/api"
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

func TestValidateTLS(t *testing.T) {
	tests := []struct {
		name           string
		route          *api.Route
		expectedErrors int
	}{
		{
			name: "No TLS Termination",
			route: &api.Route{
				TLS: &api.TLSConfig{
					Termination: "",
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Passthrough termination OK",
			route: &api.Route{
				TLS: &api.TLSConfig{
					Termination: api.TLSTerminationPassthrough,
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Reencrypt termination OK with certs",
			route: &api.Route{
				TLS: &api.TLSConfig{
					Termination:              api.TLSTerminationReencrypt,
					Certificate:              "def",
					Key:                      "ghi",
					CACertificate:            "jkl",
					DestinationCACertificate: "abc",
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Reencrypt termination OK without certs",
			route: &api.Route{
				TLS: &api.TLSConfig{
					Termination:              api.TLSTerminationReencrypt,
					DestinationCACertificate: "abc",
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Reencrypt termination no dest cert",
			route: &api.Route{
				TLS: &api.TLSConfig{
					Termination:   api.TLSTerminationReencrypt,
					Certificate:   "def",
					Key:           "ghi",
					CACertificate: "jkl",
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Edge termination OK with certs",
			route: &api.Route{
				TLS: &api.TLSConfig{
					Termination:   api.TLSTerminationEdge,
					Certificate:   "abc",
					Key:           "abc",
					CACertificate: "abc",
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Edge termination OK without certs",
			route: &api.Route{
				TLS: &api.TLSConfig{
					Termination: api.TLSTerminationEdge,
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Edge termination, dest cert",
			route: &api.Route{
				TLS: &api.TLSConfig{
					Termination:              api.TLSTerminationEdge,
					DestinationCACertificate: "abc",
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination, cert",
			route: &api.Route{
				TLS: &api.TLSConfig{Termination: api.TLSTerminationPassthrough, Certificate: "test"},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination, key",
			route: &api.Route{
				TLS: &api.TLSConfig{Termination: api.TLSTerminationPassthrough, Key: "test"},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination, ca cert",
			route: &api.Route{
				TLS: &api.TLSConfig{Termination: api.TLSTerminationPassthrough, CACertificate: "test"},
			},
			expectedErrors: 1,
		},
		{
			name: "Passthrough termination, dest ca cert",
			route: &api.Route{
				TLS: &api.TLSConfig{Termination: api.TLSTerminationPassthrough, DestinationCACertificate: "test"},
			},
			expectedErrors: 1,
		},
		{
			name: "Invalid termination type",
			route: &api.Route{
				TLS: &api.TLSConfig{
					Termination: "invalid",
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Double escaped newlines",
			route: &api.Route{
				TLS: &api.TLSConfig{
					Termination:              api.TLSTerminationReencrypt,
					Certificate:              "d\\nef",
					Key:                      "g\\nhi",
					CACertificate:            "j\\nkl",
					DestinationCACertificate: "j\\nkl",
				},
			},
			expectedErrors: 4,
		},
	}

	for _, tc := range tests {
		errs := validateTLS(tc.route)

		if len(errs) != tc.expectedErrors {
			t.Errorf("Test case %s expected %d error(s), got %d. %v", tc.name, tc.expectedErrors, len(errs), errs)
		}
	}
}
