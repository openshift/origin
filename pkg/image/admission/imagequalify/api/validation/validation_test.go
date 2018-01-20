package validation

import (
	"testing"

	"github.com/openshift/origin/pkg/image/admission/imagequalify/api"
)

func TestValidation(t *testing.T) {
	var testcases = []struct {
		description string
		config      *api.ImageQualifyConfig
		nErrors     int
	}{{
		description: "no rules",
		config:      &api.ImageQualifyConfig{},
	}, {
		description: "missing domains",
		config: &api.ImageQualifyConfig{
			Rules: []api.ImageQualifyRule{
				{
					Pattern: "a/b",
				},
				{
					Pattern: "a/b",
				},
			},
		},
		nErrors: 2,
	}, {
		description: "missing patterns",
		config: &api.ImageQualifyConfig{
			Rules: []api.ImageQualifyRule{
				{
					Domain: "foo.com",
				}, {
					Domain: "foo.com",
				},
			},
		},
		nErrors: 2,
	}, {
		description: "invalid domains",
		config: &api.ImageQualifyConfig{
			Rules: []api.ImageQualifyRule{
				{
					Domain:  "!foo!",
					Pattern: "a/b",
				},
				{
					Domain:  "[]",
					Pattern: "a/b",
				},
			},
		},
		nErrors: 2,
	}, {
		description: "invalid patterns",
		config: &api.ImageQualifyConfig{
			Rules: []api.ImageQualifyRule{
				{
					Domain:  "foo.com",
					Pattern: "!",
				},
				{
					Domain:  "bar.com",
					Pattern: "&",
				},
			},
		},
		nErrors: 2,
	}, {
		description: "valid patterns",
		config: &api.ImageQualifyConfig{
			Rules: []api.ImageQualifyRule{
				{
					Domain:  "foo.com",
					Pattern: "a/Z/*:latest-AND_greatest.@sha256:1234567890",
				},
			},
		},
	}}

	for i, tc := range testcases {
		errors := Validate(tc.config)
		nErrors := len(errors)

		if nErrors != tc.nErrors {
			t.Errorf("test #%v: %s: expected %v errors, got %v", i, tc.description, tc.nErrors, nErrors)
			for j := range errors {
				t.Errorf("test #%v: error #%v: %s", i, j, errors[j])
			}
		}
	}
}
