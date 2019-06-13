package cmd

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	templatev1 "github.com/openshift/api/template/v1"
)

func TestTransformTemplate(t *testing.T) {
	templatefoobar := &templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo_bar_template_name",
			Namespace: "foo_bar_namespace",
		},
		Parameters: []templatev1.Parameter{
			{Name: "parameter_foo_bar_exist", Value: "value_foo_bar_exist_old"},
			{Name: "parameter_foo_bar_2", Value: "value_foo_bar_2"}},
	}
	oldParameterNum := len(templatefoobar.Parameters)
	testParamMap := map[string]string{}
	testParamMap["parameter_foo_bar_exist"] = "value_foo_bar_exist_new"

	template, err := TransformTemplate(templatefoobar, fakeTemplateProcessor{}, "foo_bar_namespace", testParamMap, false)
	if err != nil {
		t.Errorf("unexpect err : %v", err)
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

type fakeTemplateProcessor struct {
}

func (fakeTemplateProcessor) Process(in *templatev1.Template) (*templatev1.Template, error) {
	return in, nil
}
