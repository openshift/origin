package cmd

import (
	"github.com/openshift/origin/pkg/client/testclient"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestTransformTemplate(t *testing.T) {
	templatefoobar := &templateapi.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo_bar_template_name",
			Namespace: "foo_bar_namespace",
		},
		Parameters: []templateapi.Parameter{
			{Name: "parameter_foo_bar_exist", Value: "value_foo_bar_exist_old"},
			{Name: "parameter_foo_bar_2", Value: "value_foo_bar_2"}},
	}
	oldParameterNum := len(templatefoobar.Parameters)
	testParamMap := map[string]string{}
	testParamMap["parameter_foo_bar_exist"] = "value_foo_bar_exist_new"

	fakeosClient := &testclient.Fake{}

	template, err := TransformTemplate(templatefoobar, fakeosClient, "foo_bar_namespace", testParamMap, false)
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
