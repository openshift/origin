package validation

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/template/api"
)

func makeParameter(name, value string) *api.Parameter {
	return &api.Parameter{
		Name:  name,
		Value: value,
	}
}

func TestValidateParameter(t *testing.T) {
	var tests = []struct {
		ParameterName   string
		IsValidExpected bool
	}{
		{"VALID_NAME", true},
		{"_valid_name_99", true},
		{"10gen_valid_name", true},
		{"", false},
		{"INVALID NAME", false},
		{"IVALID-NAME", false},
		{">INVALID_NAME", false},
		{"$INVALID_NAME", false},
		{"${INVALID_NAME}", false},
	}

	for _, test := range tests {
		param := makeParameter(test.ParameterName, "1")
		if test.IsValidExpected && len(ValidateParameter(param)) != 0 {
			t.Errorf("Expected zero validation errors on valid parameter name.")
		}
		if !test.IsValidExpected && len(ValidateParameter(param)) == 0 {
			t.Errorf("Expected some validation errors on invalid parameter name.")
		}
	}
}

func TestValidateTemplate(t *testing.T) {
	var tests = []struct {
		template        *api.Template
		isValidExpected bool
	}{
		{ // Empty Template, should fail on empty ID
			&api.Template{},
			false,
		},
		{ // Template with ID, should pass
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{Name: "templateId"},
			},
			true,
		},
		{ // Template with invalid Parameter, should fail on Parameter name
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{Name: "templateId"},
				Parameters: []api.Parameter{
					*(makeParameter("", "1")),
				},
			},
			false,
		},
		{ // Template with valid Parameter, should pass
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{Name: "templateId"},
				Parameters: []api.Parameter{
					*(makeParameter("VALID_NAME", "1")),
				},
			},
			true,
		},
		{ // Template with Item of unknown Kind, should pass
			&api.Template{
				ObjectMeta: kapi.ObjectMeta{Name: "templateId"},
				Parameters: []api.Parameter{
					*(makeParameter("VALID_NAME", "1")),
				},
				Items: []runtime.Object{},
			},
			true,
		},
	}

	for _, test := range tests {
		errs := ValidateTemplate(test.template)
		if len(errs) != 0 && test.isValidExpected {
			t.Errorf("Unexpected non-empty error list: %#v", errs)
		}
		if len(errs) == 0 && !test.isValidExpected {
			t.Errorf("Unexpected empty error list: %#v", errs)
		}
	}
}
