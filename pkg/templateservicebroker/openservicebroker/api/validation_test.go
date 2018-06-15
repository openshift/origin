package api

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

const validUUID = "fe6e44ea-377a-457c-9fa1-ba06ad356839"

func TestValidateProvisionRequest(t *testing.T) {
	tests := []struct {
		name              string
		preq              ProvisionRequest
		expectErrorPrefix string
	}{
		{
			name: "missing context platform",
			preq: ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Context: KubernetesContext{
					Namespace: "test",
				},
			},
			expectErrorPrefix: `context.platform: Required value`,
		},
		{
			name: "bad context platform",
			preq: ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Context: KubernetesContext{
					Platform:  "b@d",
					Namespace: "test",
				},
			},
			expectErrorPrefix: `context.platform: Invalid value: "b@d": must equal kubernetes`,
		},
		{
			name: "missing context namespace",
			preq: ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Context: KubernetesContext{
					Platform: ContextPlatformKubernetes,
				},
			},
			expectErrorPrefix: `context.namespace: Required value`,
		},
		{
			name: "bad context namespace",
			preq: ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Context: KubernetesContext{
					Platform:  ContextPlatformKubernetes,
					Namespace: "b@d",
				},
			},
			expectErrorPrefix: `context.namespace: Invalid value: "b@d": `, // a DNS-1123 label must consist of ...
		},
		{
			name: "empty ServiceID",
			preq: ProvisionRequest{
				PlanID: validUUID,
				Context: KubernetesContext{
					Platform:  ContextPlatformKubernetes,
					Namespace: "test",
				},
			},
			expectErrorPrefix: `service_id: Invalid value: "": must be a valid UUID`,
		},
		{
			name: "bad ServiceID",
			preq: ProvisionRequest{
				ServiceID: "bad",
				PlanID:    validUUID,
				Context: KubernetesContext{
					Platform:  ContextPlatformKubernetes,
					Namespace: "test",
				},
			},
			expectErrorPrefix: `service_id: Invalid value: "bad": must be a valid UUID`,
		},
		{
			name: "empty PlanID",
			preq: ProvisionRequest{
				ServiceID: validUUID,
				Context: KubernetesContext{
					Platform:  ContextPlatformKubernetes,
					Namespace: "test",
				},
			},
			expectErrorPrefix: `plan_id: Invalid value: "": must be a valid UUID`,
		},
		{
			name: "bad PlanID",
			preq: ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    "bad",
				Context: KubernetesContext{
					Platform:  ContextPlatformKubernetes,
					Namespace: "test",
				},
			},
			expectErrorPrefix: `plan_id: Invalid value: "bad": must be a valid UUID`,
		},
		{
			name: "good",
			preq: ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Context: KubernetesContext{
					Platform:  ContextPlatformKubernetes,
					Namespace: "test",
				},
			},
			expectErrorPrefix: ``,
		},
	}

	for _, test := range tests {
		errors := ValidateProvisionRequest(&test.preq)
		if test.expectErrorPrefix == "" {
			if len(errors) > 0 {
				t.Errorf("%q: expectErrorPrefix was %q but errors was %q", test.name, test.expectErrorPrefix, errors)
			}
		} else {
			found := false
			for _, err := range errors {
				if strings.HasPrefix(err.Error(), test.expectErrorPrefix) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%q: expectErrorPrefix was %q but errors was %q", test.name, test.expectErrorPrefix, errors)
			}
		}
	}
}

func TestValidateBindRequest(t *testing.T) {
	tests := []struct {
		name        string
		breq        BindRequest
		expectError string
	}{
		{
			name: "empty ServiceID",
			breq: BindRequest{
				PlanID: validUUID,
			},
			expectError: `service_id: Invalid value: "": must be a valid UUID`,
		},
		{
			name: "bad ServiceID",
			breq: BindRequest{
				ServiceID: "bad",
				PlanID:    validUUID,
			},
			expectError: `service_id: Invalid value: "bad": must be a valid UUID`,
		},
		{
			name: "empty PlanID",
			breq: BindRequest{
				ServiceID: validUUID,
			},
			expectError: `plan_id: Invalid value: "": must be a valid UUID`,
		},
		{
			name: "bad PlanID",
			breq: BindRequest{
				ServiceID: validUUID,
				PlanID:    "bad",
			},
			expectError: `plan_id: Invalid value: "bad": must be a valid UUID`,
		},
		{
			name: "good",
			breq: BindRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
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

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name        string
		uuid        string
		expectError string
	}{
		{
			name:        "empty UUID",
			uuid:        "",
			expectError: `uuid: Invalid value: "": must be a valid UUID`,
		},
		{
			name:        "bad UUID",
			uuid:        "bad",
			expectError: `uuid: Invalid value: "bad": must be a valid UUID`,
		},
		{
			name:        "good",
			uuid:        validUUID,
			expectError: ``,
		},
	}

	for _, test := range tests {
		errors := ValidateUUID(field.NewPath("uuid"), test.uuid)
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
