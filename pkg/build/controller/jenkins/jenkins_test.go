package jenkins

import (
	"fmt"
	"testing"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

func TestSubstituteTemplateParameters(t *testing.T) {
	template := &templateapi.Template{
		Parameters: []templateapi.Parameter{
			{Name: "parameter_foo_bar_exist", Value: "value_foo_bar_exist_old"},
			{Name: "parameter_foo_bar_2", Value: "value_foo_bar_2"}},
	}
	oldParameterNum := len(template.Parameters)
	testParamMap := map[string]string{}
	testParamMap["parameter_foo_bar_exist"] = "value_foo_bar_exist_new"
	testParamMap["parameter_foo_bar_no_exist"] = "value_foo_bar_no_exist"
	testParamMap[""] = "value_foo_bar_empty"

	errors := substituteTemplateParameters(testParamMap, template)
	if len(errors) != 2 {
		for index, err := range errors {
			fmt.Printf("errors[%d] : %v\n", index, err)
		}
		t.Errorf("expect unknown parameter and empty errors")
	}
	if len(template.Parameters) != oldParameterNum {
		t.Errorf("expect parameters num : %d", oldParameterNum)
	}
	if !(template.Parameters[0].Name == "parameter_foo_bar_exist" &&
		template.Parameters[0].Value == "value_foo_bar_exist_new") {
		t.Errorf("expect Name : %q Value : %q get Name : %q Value : %q",
			"parameter_foo_bar_exist", "value_foo_bar_exist_new", template.Parameters[0].Name, template.Parameters[0].Value)
	}
	if !(template.Parameters[1].Name == "parameter_foo_bar_2" &&
		template.Parameters[1].Value == "value_foo_bar_2") {
		t.Errorf("expect Name : %q Value : %q get Name : %q Value : %q",
			"parameter_foo_bar_2", "value_foo_bar_2", template.Parameters[1].Name, template.Parameters[1].Value)
	}
}
