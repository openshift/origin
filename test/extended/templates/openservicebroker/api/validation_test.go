package api

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

const validUUID = "fe6e44ea-377a-457c-9fa1-ba06ad356839"

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
