package servicebroker

import (
	"fmt"
	"testing"

	"github.com/openshift/origin/pkg/openservicebroker/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

const validUUID = "4f8a47f7-900f-48b4-aad1-865760feaa04"

func TestValidateProvisionRequest(t *testing.T) {
	tests := []struct {
		name        string
		preq        api.ProvisionRequest
		expectError string
	}{
		{
			name: "missing RequesterUsernameParameterKey key",
			preq: api.ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Parameters: map[string]string{
					templateapi.NamespaceParameterKey: "test",
				},
			},
			expectError: `parameters.` + templateapi.RequesterUsernameParameterKey + `: Required value`,
		},
		{
			name: "missing NamespaceParameterKey key",
			preq: api.ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Parameters: map[string]string{
					templateapi.RequesterUsernameParameterKey: "test",
				},
			},
			expectError: `parameters.` + templateapi.NamespaceParameterKey + `: Required value`,
		},
		{
			name: "bad key",
			preq: api.ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Parameters: map[string]string{
					"b@d": "",
					templateapi.NamespaceParameterKey:         "test",
					templateapi.RequesterUsernameParameterKey: "test",
				},
			},
			expectError: `parameters.b@d: Invalid value: "b@d": does not match ^[a-zA-Z0-9_]+$`,
		},
		{
			name: "good",
			preq: api.ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Parameters: map[string]string{
					"azAZ09_":                                 "",
					templateapi.NamespaceParameterKey:         "test",
					templateapi.RequesterUsernameParameterKey: "test",
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
			name: "missing RequesterUsernameParameterKey key",
			breq: api.BindRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
			},
			expectError: `parameters.` + templateapi.RequesterUsernameParameterKey + `: Required value`,
		},
		{
			name: "bad key 1",
			breq: api.BindRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Parameters: map[string]string{
					"b@d": "",
					templateapi.RequesterUsernameParameterKey: "test",
				},
			},
			expectError: `parameters.b@d: Invalid value: "b@d": does not match ^[a-zA-Z0-9_]+$`,
		},
		{
			name: "bad key 2",
			breq: api.BindRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Parameters: map[string]string{
					templateapi.NamespaceParameterKey:         "test",
					templateapi.RequesterUsernameParameterKey: "test",
				},
			},
			expectError: fmt.Sprintf(`parameters.%[1]s: Invalid value: "%[1]s": does not match ^[a-zA-Z0-9_]+$`, templateapi.NamespaceParameterKey),
		},
		{
			name: "good",
			breq: api.BindRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Parameters: map[string]string{
					"azAZ09_": "",
					templateapi.RequesterUsernameParameterKey: "test",
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
