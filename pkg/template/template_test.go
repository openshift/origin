package template

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"

	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
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

func ExampleProcessTemplateParameters() {
	var template api.Template
	jsonData, _ := ioutil.ReadFile("../../test/templates/fixtures/guestbook.json")
	latest.Codec.DecodeInto(jsonData, &template)

	generators := map[string]generator.Generator{
		"expression": generator.NewExpressionValueGenerator(rand.New(rand.NewSource(1337))),
	}
	processor := NewProcessor(generators)

	// Define custom parameter for the transformation:
	AddParameter(&template, makeParameter("CUSTOM_PARAM1", "1", ""))

	// Transform the template config into the result config
	config, err := processor.Process(&template)
	fmt.Println(errors.NewAggregate(err))
	if config != nil {
		result, err := latest.Codec.Encode(config)
		if err != nil {
			fmt.Printf("Unexpected error during encoding Config: %#v", err)
		}
		fmt.Println(string(result))
	}
	// Output:
	//<nil>
	//{"kind":"Config","apiVersion":"v1beta1","metadata":{"creationTimestamp":null},"items":[{"kind":"Route","apiVersion":"v1beta1","metadata":{"name":"frontend-route","creationTimestamp":null},"host":"guestbook.example.com","serviceName":"frontend-service"},{"kind":"Service","id":"frontend-service","creationTimestamp":null,"apiVersion":"v1beta1","port":5432,"protocol":"TCP","selector":{"name":"frontend-service"},"containerPort":0,"sessionAffinity":"None"},{"kind":"Service","id":"redis-master","creationTimestamp":null,"apiVersion":"v1beta1","port":10000,"protocol":"TCP","selector":{"name":"redis-master"},"containerPort":0,"sessionAffinity":"None"},{"kind":"Service","id":"redis-slave","creationTimestamp":null,"apiVersion":"v1beta1","port":10001,"protocol":"TCP","selector":{"name":"redis-slave"},"containerPort":0,"sessionAffinity":"None"},{"kind":"Pod","id":"redis-master","creationTimestamp":null,"apiVersion":"v1beta1","labels":{"name":"redis-master"},"desiredState":{"manifest":{"version":"v1beta2","id":"","volumes":null,"containers":[{"name":"master","image":"dockerfile/redis","ports":[{"containerPort":6379,"protocol":"TCP"}],"env":[{"name":"REDIS_PASSWORD","key":"REDIS_PASSWORD","value":"P8vxbV4C"}],"resources":{},"terminationMessagePath":"/dev/termination-log","imagePullPolicy":"PullIfNotPresent","capabilities":{}}],"restartPolicy":{"always":{}},"dnsPolicy":"ClusterFirst"}},"currentState":{"manifest":{"version":"","id":"","volumes":null,"containers":null,"restartPolicy":{}}}},{"kind":"ReplicationController","id":"guestbook","creationTimestamp":null,"apiVersion":"v1beta1","desiredState":{"replicas":3,"replicaSelector":{"name":"frontend-service"},"podTemplate":{"desiredState":{"manifest":{"version":"v1beta2","id":"","volumes":null,"containers":[{"name":"php-redis","image":"brendanburns/php-redis","ports":[{"hostPort":8000,"containerPort":80,"protocol":"TCP"}],"env":[{"name":"ADMIN_USERNAME","key":"ADMIN_USERNAME","value":"adminQ3H"},{"name":"ADMIN_PASSWORD","key":"ADMIN_PASSWORD","value":"dwNJiJwW"},{"name":"REDIS_PASSWORD","key":"REDIS_PASSWORD","value":"P8vxbV4C"}],"resources":{},"terminationMessagePath":"/dev/termination-log","imagePullPolicy":"PullIfNotPresent","capabilities":{}}],"restartPolicy":{"always":{}},"dnsPolicy":"ClusterFirst"}},"labels":{"name":"frontend-service"}}},"currentState":{"replicas":0,"podTemplate":{"desiredState":{"manifest":{"version":"","id":"","volumes":null,"containers":null,"restartPolicy":{}}}}}},{"kind":"ReplicationController","id":"redis-slave","creationTimestamp":null,"apiVersion":"v1beta1","desiredState":{"replicas":2,"replicaSelector":{"name":"redis-slave"},"podTemplate":{"desiredState":{"manifest":{"version":"v1beta2","id":"","volumes":null,"containers":[{"name":"slave","image":"brendanburns/redis-slave","ports":[{"hostPort":6380,"containerPort":6379,"protocol":"TCP"}],"env":[{"name":"REDIS_PASSWORD","key":"REDIS_PASSWORD","value":"P8vxbV4C"}],"resources":{},"terminationMessagePath":"/dev/termination-log","imagePullPolicy":"PullIfNotPresent","capabilities":{}}],"restartPolicy":{"always":{}},"dnsPolicy":"ClusterFirst"}},"labels":{"name":"redis-slave"}}},"currentState":{"replicas":0,"podTemplate":{"desiredState":{"manifest":{"version":"","id":"","volumes":null,"containers":null,"restartPolicy":{}}}}}}]}
}
