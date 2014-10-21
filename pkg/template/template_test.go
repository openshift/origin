package template

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/origin/pkg/template/generator"
)

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
	processor.AddParameter(&template, api.Parameter{Name: "CUSTOM_PARAM", Value: "1"})
	processor.AddParameter(&template, api.Parameter{Name: "CUSTOM_PARAM", Value: "2"})

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
			api.Parameter{Name: "PARAM", Generate: "", Value: "X"},
			map[string]generator.Generator{},
			true,
			api.Parameter{Name: "PARAM", Generate: "", Value: "X"},
		},
		{ // Foo generator, should pass
			api.Parameter{Name: "PARAM", Generate: "foo", Value: ""},
			map[string]generator.Generator{"foo": FooGenerator{}},
			true,
			api.Parameter{Name: "PARAM", Generate: "", Value: "foo"},
		},
		{ // Invalid generator, should fail
			api.Parameter{Name: "PARAM", Generate: "invalid", Value: ""},
			map[string]generator.Generator{"invalid": nil},
			false,
			api.Parameter{Name: "PARAM", Generate: "invalid", Value: ""},
		},
		{ // Error generator, should fail
			api.Parameter{Name: "PARAM", Generate: "error", Value: ""},
			map[string]generator.Generator{"error": ErrorGenerator{}},
			false,
			api.Parameter{Name: "PARAM", Generate: "error", Value: ""},
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
	processor.AddParameter(&template, api.Parameter{Name: "CUSTOM_PARAM1", Value: "1"})

	// Transform the template config into the result config
	config, _ := processor.Process(&template)
	// Reset the timestamp for the output comparison
	config.CreationTimestamp = util.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

	result, _ := latest.Codec.Encode(config)
	fmt.Println(string(result))
	// Output:
	//{"kind":"Config","id":"guestbook","creationTimestamp":"1980-01-01T00:00:00Z","apiVersion":"v1beta1","name":"guestbook-example","description":"Example shows how to build a simple multi-tier application using Kubernetes and Docker","items":[{"kind":"Route","id":"frontendroute","creationTimestamp":null,"apiVersion":"v1beta1","host":"guestbook.example.com","serviceName":"frontend","labels":{"name":"frontend"}},{"kind":"Service","id":"frontend","creationTimestamp":null,"apiVersion":"v1beta1","port":5432,"selector":{"name":"frontend"},"containerPort":0},{"kind":"Service","id":"redismaster","creationTimestamp":null,"apiVersion":"v1beta1","port":10000,"selector":{"name":"redis-master"},"containerPort":0},{"kind":"Service","id":"redisslave","creationTimestamp":null,"apiVersion":"v1beta1","port":10001,"labels":{"name":"redisslave"},"selector":{"name":"redisslave"},"containerPort":0},{"kind":"Pod","id":"redis-master-2","creationTimestamp":null,"apiVersion":"v1beta1","labels":{"name":"redis-master"},"desiredState":{"manifest":{"version":"v1beta1","id":"redis-master-2","volumes":null,"containers":[{"name":"master","image":"dockerfile/redis","ports":[{"containerPort":6379}],"env":[{"name":"REDIS_PASSWORD","key":"REDIS_PASSWORD","value":"P8vxbV4C"}],"imagePullPolicy":""}],"restartPolicy":{}}},"currentState":{"manifest":{"version":"","id":"","volumes":null,"containers":null,"restartPolicy":{}}}},{"kind":"ReplicationController","id":"frontendController","creationTimestamp":null,"apiVersion":"v1beta1","desiredState":{"replicas":3,"replicaSelector":{"name":"frontend"},"podTemplate":{"desiredState":{"manifest":{"version":"v1beta1","id":"frontendController","volumes":null,"containers":[{"name":"php-redis","image":"brendanburns/php-redis","ports":[{"hostPort":8000,"containerPort":80}],"env":[{"name":"ADMIN_USERNAME","key":"ADMIN_USERNAME","value":"adminQ3H"},{"name":"ADMIN_PASSWORD","key":"ADMIN_PASSWORD","value":"dwNJiJwW"},{"name":"REDIS_PASSWORD","key":"REDIS_PASSWORD","value":"P8vxbV4C"}],"imagePullPolicy":""}],"restartPolicy":{}}},"labels":{"name":"frontend"}}},"currentState":{"replicas":0,"podTemplate":{"desiredState":{"manifest":{"version":"","id":"","volumes":null,"containers":null,"restartPolicy":{}}}}},"labels":{"name":"frontend"}},{"kind":"ReplicationController","id":"redisSlaveController","creationTimestamp":null,"apiVersion":"v1beta1","desiredState":{"replicas":2,"replicaSelector":{"name":"redisslave"},"podTemplate":{"desiredState":{"manifest":{"version":"v1beta1","id":"redisSlaveController","volumes":null,"containers":[{"name":"slave","image":"brendanburns/redis-slave","ports":[{"hostPort":6380,"containerPort":6379}],"env":[{"name":"REDIS_PASSWORD","key":"REDIS_PASSWORD","value":"P8vxbV4C"}],"imagePullPolicy":""}],"restartPolicy":{}}},"labels":{"name":"redisslave"}}},"currentState":{"replicas":0,"podTemplate":{"desiredState":{"manifest":{"version":"","id":"","volumes":null,"containers":null,"restartPolicy":{}}}}},"labels":{"name":"redisslave"}}]}
}
