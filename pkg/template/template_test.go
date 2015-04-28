package template

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strings"
	"testing"

	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/origin/pkg/template/generator"
)

func makeParameter(name, value, generate string) api.Parameter {
	return api.Parameter{
		Name:     name,
		Value:    value,
		Generate: generate,
	}
}

func TestAddParameter(t *testing.T) {
	var template api.Template

	jsonData, _ := ioutil.ReadFile("../../test/templates/fixtures/guestbook.json")
	json.Unmarshal(jsonData, &template)

	AddParameter(&template, makeParameter("CUSTOM_PARAM", "1", ""))
	AddParameter(&template, makeParameter("CUSTOM_PARAM", "2", ""))

	if p := GetParameterByName(&template, "CUSTOM_PARAM"); p == nil {
		t.Errorf("Unable to add a custom parameter to the template")
	} else {
		if p.Value != "2" {
			t.Errorf("Unable to replace the custom parameter value in template")
		}
	}
}

type FooGenerator struct {
}

func (g FooGenerator) GenerateValue(expression string) (interface{}, error) {
	return "foo", nil
}

type ErrorGenerator struct {
}

func (g ErrorGenerator) GenerateValue(expression string) (interface{}, error) {
	return "", fmt.Errorf("error")
}

func TestParameterGenerators(t *testing.T) {
	tests := []struct {
		parameter  api.Parameter
		generators map[string]generator.Generator
		shouldPass bool
		expected   api.Parameter
	}{
		{ // Empty generator, should pass
			makeParameter("PARAM", "X", ""),
			map[string]generator.Generator{},
			true,
			makeParameter("PARAM", "X", ""),
		},
		{ // Foo generator, should pass
			makeParameter("PARAM", "", "foo"),
			map[string]generator.Generator{"foo": FooGenerator{}},
			true,
			makeParameter("PARAM", "foo", ""),
		},
		{ // Invalid generator, should fail
			makeParameter("PARAM", "", "invalid"),
			map[string]generator.Generator{"invalid": nil},
			false,
			makeParameter("PARAM", "", "invalid"),
		},
		{ // Error generator, should fail
			makeParameter("PARAM", "", "error"),
			map[string]generator.Generator{"error": ErrorGenerator{}},
			false,
			makeParameter("PARAM", "", "error"),
		},
	}

	for i, test := range tests {
		processor := NewProcessor(test.generators)
		template := api.Template{Parameters: []api.Parameter{test.parameter}}
		err := processor.GenerateParameterValues(&template)
		if err != nil && test.shouldPass {
			t.Errorf("test[%v]: Unexpected error %v", i, err)
		}
		if err == nil && !test.shouldPass {
			t.Errorf("test[%v]: Expected error", i)
		}
		actual := template.Parameters[0]
		if actual.Value != test.expected.Value {
			t.Errorf("test[%v]: Unexpected value: Expected: %#v, got: %#v", i, test.expected.Value, test.parameter.Value)
		}
	}
}

func TestProcessValueEscape(t *testing.T) {
	var template api.Template
	if err := latest.Codec.DecodeInto([]byte(`{
		"kind":"Template", "apiVersion":"v1beta1",
		"items": [
			{
				"kind": "Service", "apiVersion": "v1beta3${VALUE}",
				"metadata": {
					"labels": {
						"key1": "${VALUE}",
						"key2": "$${VALUE}"
					}
				}
			}
		]
	}`), &template); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	generators := map[string]generator.Generator{
		"expression": generator.NewExpressionValueGenerator(rand.New(rand.NewSource(1337))),
	}
	processor := NewProcessor(generators)

	// Define custom parameter for the transformation:
	AddParameter(&template, makeParameter("VALUE", "1", ""))

	// Transform the template config into the result config
	config, errs := processor.Process(&template)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	result, err := latest.Codec.Encode(config)
	if err != nil {
		t.Fatalf("unexpected error during encoding Config: %#v", err)
	}
	expect := `{"kind":"Config","apiVersion":"v1beta1","metadata":{"creationTimestamp":null},"items":[{"apiVersion":"v1beta31","kind":"Service","metadata":{"labels":{"key1":"1","key2":"$1"}}}]}`
	if expect != string(result) {
		t.Errorf("unexpected output: %s", util.StringDiff(expect, string(result)))
	}
}

func TestProcessTemplateParameters(t *testing.T) {
	var template api.Template
	jsonData, _ := ioutil.ReadFile("../../test/templates/fixtures/guestbook.json")
	if err := latest.Codec.DecodeInto(jsonData, &template); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	generators := map[string]generator.Generator{
		"expression": generator.NewExpressionValueGenerator(rand.New(rand.NewSource(1337))),
	}
	processor := NewProcessor(generators)

	// Define custom parameter for the transformation:
	AddParameter(&template, makeParameter("CUSTOM_PARAM1", "1", ""))

	// Transform the template config into the result config
	config, errs := processor.Process(&template)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	result, err := latest.Codec.Encode(config)
	if err != nil {
		t.Fatalf("unexpected error during encoding Config: %#v", err)
	}
	expect := `{"kind":"Config","apiVersion":"v1beta1","metadata":{"creationTimestamp":null},"items":[{"apiVersion":"v1beta1","host":"guestbook.example.com","id":"frontend-route","kind":"Route","metadata":{"name":"frontend-route"},"serviceName":"frontend-service"},{"apiVersion":"v1beta1","id":"frontend-service","kind":"Service","port":5432,"selector":{"name":"frontend-service"}},{"apiVersion":"v1beta1","id":"redis-master","kind":"Service","port":10000,"selector":{"name":"redis-master"}},{"apiVersion":"v1beta1","id":"redis-slave","kind":"Service","port":10001,"selector":{"name":"redis-slave"}},{"apiVersion":"v1beta1","desiredState":{"manifest":{"containers":[{"env":[{"name":"REDIS_PASSWORD","value":"P8vxbV4C"}],"image":"dockerfile/redis","name":"master","ports":[{"containerPort":6379}]}],"name":"redis-master","version":"v1beta1"}},"id":"redis-master","kind":"Pod","labels":{"name":"redis-master"}},{"apiVersion":"v1beta1","desiredState":{"podTemplate":{"desiredState":{"manifest":{"containers":[{"env":[{"name":"ADMIN_USERNAME","value":"adminQ3H"},{"name":"ADMIN_PASSWORD","value":"dwNJiJwW"},{"name":"REDIS_PASSWORD","value":"P8vxbV4C"}],"image":"brendanburns/php-redis","name":"php-redis","ports":[{"containerPort":80,"hostPort":8000}]}],"name":"guestbook","version":"v1beta1"}},"labels":{"name":"frontend-service"}},"replicaSelector":{"name":"frontend-service"},"replicas":3},"id":"guestbook","kind":"ReplicationController"},{"apiVersion":"v1beta1","desiredState":{"podTemplate":{"desiredState":{"manifest":{"containers":[{"env":[{"name":"REDIS_PASSWORD","value":"P8vxbV4C"}],"image":"brendanburns/redis-slave","name":"slave","ports":[{"containerPort":6379,"hostPort":6380}]}],"id":"redis-slave","version":"v1beta1"}},"labels":{"name":"redis-slave"}},"replicaSelector":{"name":"redis-slave"},"replicas":2},"id":"redis-slave","kind":"ReplicationController"}]}`
	expect = strings.Replace(expect, "\n", "", -1)
	if string(result) != expect {
		t.Errorf("unexpected output: %s", util.StringDiff(expect, string(result)))
	}
}
