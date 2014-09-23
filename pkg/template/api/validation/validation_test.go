package validation

import (
	goruntime "runtime"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/template/api"
)

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
		param := &api.Parameter{Name: test.ParameterName, Value: "1"}
		if test.IsValidExpected && len(ValidateParameter(param)) != 0 {
			t.Errorf("Expected zero validation errors on valid parameter name.")
		}
		if !test.IsValidExpected && len(ValidateParameter(param)) == 0 {
			t.Errorf("Expected some validation errors on invalid parameter name.")
		}
	}
}

func TestValidateTemplate(t *testing.T) {
	shouldPass := func(template *api.Template) {
		errs := ValidateTemplate(template)
		if len(errs) != 0 {
			_, _, line, _ := goruntime.Caller(1)
			t.Errorf("line %v: Unexpected non-zero error list: %#v", line, errs)
		}
	}
	shouldFail := func(template *api.Template) {
		if len(ValidateTemplate(template)) == 0 {
			_, _, line, _ := goruntime.Caller(1)
			t.Errorf("line %v: Expected non-zero error list", line)
		}
	}

	// Test empty Template, should fail on empty ID
	template := &api.Template{}
	shouldFail(template)

	// Set ID, should pass
	template.JSONBase.ID = "templateId"
	shouldPass(template)

	// Add invalid Parameter, should fail on Parameter name
	template.Parameters = []api.Parameter{{Name: "", Value: "1"}}
	shouldFail(template)

	// Fix Parameter name, should pass
	template.Parameters[0].Name = "VALID_NAME"
	shouldPass(template)

	// Add Item of unknown Kind, should pass
	template.Items = []runtime.EmbeddedObject{{}}
	shouldPass(template)
}
