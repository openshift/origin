package template

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/origin/pkg/template/generator"
)

func makeParameter(name, value, generate string) api.Parameter {
	return api.Parameter{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Value:    value,
		Generate: generate,
	}
}

func TestNewTemplate(t *testing.T) {
	var template api.Template

	jsonData, _ := ioutil.ReadFile("../../examples/guestbook/template.json")
	if err := json.Unmarshal(jsonData, &template); err != nil {
		t.Errorf("Unable to process the JSON template file: %v", err)
	}
}

func TestAddParameter(t *testing.T) {
	var template api.Template

	jsonData, _ := ioutil.ReadFile("../../examples/guestbook/template.json")
	json.Unmarshal(jsonData, &template)

	processor := NewTemplateProcessor(nil)
	processor.AddParameter(&template, makeParameter("CUSTOM_PARAM", "1", ""))
	processor.AddParameter(&template, makeParameter("CUSTOM_PARAM", "2", ""))

	if p := processor.GetParameterByName(&template, "CUSTOM_PARAM"); p == nil {
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
		processor := NewTemplateProcessor(test.generators)
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

func ExampleProcessTemplateParameters() {
	var template api.Template
	jsonData, _ := ioutil.ReadFile("../../examples/guestbook/template.json")
	latest.Codec.DecodeInto(jsonData, &template)

	generators := map[string]generator.Generator{
		"expression": generator.NewExpressionValueGenerator(rand.New(rand.NewSource(1337))),
	}
	processor := NewTemplateProcessor(generators)

	// Define custom parameter for the transformation:
	processor.AddParameter(&template, makeParameter("CUSTOM_PARAM1", "1", ""))

	// Transform the template config into the result config
	config, err := processor.Process(&template)

	if err != nil {
		fmt.Printf("%+v", err)
	} else {
		// Reset the timestamp for the output comparison
		config.ObjectMeta.CreationTimestamp = util.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

		result, err := latest.Codec.Encode(config)
		if err != nil {
			fmt.Printf("Unexpected error during encoding Config: %#v", err)
		}
		fmt.Println(string(result))
	}
	// Output:
	// {"kind":"Config","apiVersion":"v1beta1","name":"guestbook-example","creationTimestamp":"1980-01-01T00:00:00Z","annotations":{"description":"Example shows how to build a simple multi-tier application using Kubernetes and Docker"},"items":[{"kind":"Route","apiVersion":"v1beta1","name":"frontend-route","creationTimestamp":null,"host":"guestbook.example.com","serviceName":"frontend"},{"kind":"Service","creationTimestamp":null,"apiVersion":"v1beta1","port":5432,"selector":{"name":"guestbook"},"containerPort":0},{"kind":"Service","creationTimestamp":null,"apiVersion":"v1beta1","port":10000,"selector":{"name":"redis-master"},"containerPort":0},{"kind":"Service","creationTimestamp":null,"apiVersion":"v1beta1","port":10001,"selector":{"name":"redis-slave"},"containerPort":0},{"kind":"Pod","creationTimestamp":null,"apiVersion":"v1beta1","desiredState":{"manifest":{"version":"v1beta1","id":"","volumes":null,"containers":[{"name":"master","image":"dockerfile/redis","ports":[{"containerPort":6379}],"env":[{"name":"REDIS_PASSWORD","key":"REDIS_PASSWORD","value":"P8vxbV4C"}],"imagePullPolicy":""}],"restartPolicy":{}}},"currentState":{"manifest":{"version":"","id":"","volumes":null,"containers":null,"restartPolicy":{}}}},{"kind":"ReplicationController","creationTimestamp":null,"apiVersion":"v1beta1","desiredState":{"replicas":3,"replicaSelector":{"name":"frontend"},"podTemplate":{"desiredState":{"manifest":{"version":"v1beta1","id":"","volumes":null,"containers":[{"name":"php-redis","image":"brendanburns/php-redis","ports":[{"hostPort":8000,"containerPort":80}],"env":[{"name":"ADMIN_USERNAME","key":"ADMIN_USERNAME","value":"adminQ3H"},{"name":"ADMIN_PASSWORD","key":"ADMIN_PASSWORD","value":"dwNJiJwW"},{"name":"REDIS_PASSWORD","key":"REDIS_PASSWORD","value":"P8vxbV4C"}],"imagePullPolicy":""}],"restartPolicy":{}}}}},"currentState":{"replicas":0,"podTemplate":{"desiredState":{"manifest":{"version":"","id":"","volumes":null,"containers":null,"restartPolicy":{}}}}}},{"kind":"ReplicationController","creationTimestamp":null,"apiVersion":"v1beta1","desiredState":{"replicas":2,"replicaSelector":{"name":"redis-slave"},"podTemplate":{"desiredState":{"manifest":{"version":"v1beta1","id":"redis-slave","volumes":null,"containers":[{"name":"slave","image":"brendanburns/redis-slave","ports":[{"hostPort":6380,"containerPort":6379}],"env":[{"name":"REDIS_PASSWORD","key":"REDIS_PASSWORD","value":"P8vxbV4C"}],"imagePullPolicy":""}],"restartPolicy":{}}}}},"currentState":{"replicas":0,"podTemplate":{"desiredState":{"manifest":{"version":"","id":"","volumes":null,"containers":null,"restartPolicy":{}}}}}}]}
}
