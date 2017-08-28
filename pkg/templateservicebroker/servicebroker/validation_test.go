package servicebroker

import (
	"testing"

	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
)

const validUUID = "4f8a47f7-900f-48b4-aad1-865760feaa04"

func TestValidateProvisionRequest(t *testing.T) {
	tests := []struct {
		name        string
		preq        api.ProvisionRequest
		expectError string
	}{
		{
			name: "bad key",
			preq: api.ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Context: api.KubernetesContext{
					Platform:  api.ContextPlatformKubernetes,
					Namespace: "test",
				},
				Parameters: map[string]string{
					"b@d": "",
				},
			},
			expectError: `parameters.b@d: Invalid value: "b@d": does not match ^[a-zA-Z0-9_]+$`,
		},
		{
			name: "good",
			preq: api.ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Context: api.KubernetesContext{
					Platform:  api.ContextPlatformKubernetes,
					Namespace: "test",
				},
				Parameters: map[string]string{
					"azAZ09_": "",
				},
			},
			expectError: ``,
		},
	}

	for _, test := range tests {
		errors := ValidateProvisionRequest(&test.preq)
		if test.expectError == "" {
			if len(errors) > 0 {
				t.Errorf("%q: expectError was %q but errors was %q", test.name, test.expectError, errors)
			}
		} else {
			found := false
			for _, err := range errors {
				if err.Error() == test.expectError {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%q: expectError was %q but errors was %q", test.name, test.expectError, errors)
			}
		}
	}
}

func TestValidateBindRequest(t *testing.T) {
	tests := []struct {
		name        string
		breq        api.BindRequest
		expectError string
	}{
		{
			name: "bad key",
			breq: api.BindRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Parameters: map[string]string{
					"b@d": "",
				},
			},
			expectError: `parameters.b@d: Invalid value: "b@d": does not match ^[a-zA-Z0-9_]+$`,
		},
		{
			name: "good",
			breq: api.BindRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Parameters: map[string]string{
					"azAZ09_": "",
				},
			},
			expectError: ``,
		},
	}

	for _, test := range tests {
		errors := ValidateBindRequest(&test.breq)
		if test.expectError == "" {
			if len(errors) > 0 {
				t.Errorf("%q: expectError was %q but errors was %q", test.name, test.expectError, errors)
			}
		} else {
			found := false
			for _, err := range errors {
				if err.Error() == test.expectError {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%q: expectError was %q but errors was %q", test.name, test.expectError, errors)
			}
		}
	}
}
