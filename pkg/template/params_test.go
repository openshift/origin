package template

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

func makeParameter(name, value, generate string, required bool) templateapi.Parameter {
	return templateapi.Parameter{
		Name:     name,
		Value:    value,
		Generate: generate,
		Required: required,
	}
}

func TestAddParameter(t *testing.T) {
	var template templateapi.Template

	jsonData, _ := ioutil.ReadFile("../../test/templates/testdata/guestbook.json")
	json.Unmarshal(jsonData, &template)

	AddParameter(&template, makeParameter("CUSTOM_PARAM", "1", "", false))
	AddParameter(&template, makeParameter("CUSTOM_PARAM", "2", "", false))

	if p := GetParameterByName(&template, "CUSTOM_PARAM"); p == nil {
		t.Errorf("Unable to add a custom parameter to the template")
	} else {
		if p.Value != "2" {
			t.Errorf("Unable to replace the custom parameter value in template")
		}
	}
}
