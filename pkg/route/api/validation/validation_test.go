package validation

import (
	"testing"

	"github.com/openshift/origin/pkg/route/api"
)

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
