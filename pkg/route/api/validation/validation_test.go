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
		DestinationCACertificate: "abc",
		Certificate:              "def",
		Key:                      "ghi",
		CACertificate:            "jkl",
	})

	if len(errs) > 0 {
		t.Errorf("Unexpected non-empty error list: %#v", errs)
	}
}

func TestValidateTLSEdgeTermOKCerts(t *testing.T) {
	errs := validateTLS(&api.TLSConfig{
		Termination:   api.TLSTerminationEdge,
		Certificate:   "abc",
		Key:           "abc",
		CACertificate: "abc",
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
