package template

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"regexp"
	"strings"
	"testing"

	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta3"
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
	errs := processor.Process(&template)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	result, err := v1beta3.Codec.Encode(&template)
	if err != nil {
		t.Fatalf("unexpected error during encoding Config: %#v", err)
	}
	expect := `{"kind":"Template","apiVersion":"v1beta3","metadata":{"creationTimestamp":null},"objects":[{"apiVersion":"v1beta31","kind":"Service","metadata":{"labels":{"key1":"1","key2":"$1"}}}],"parameters":[{"name":"VALUE","value":"1"}]}`
	if expect != string(result) {
		t.Errorf("unexpected output: %s", util.StringDiff(expect, string(result)))
	}
}

var trailingWhitespace = regexp.MustCompile(`\n\s*`)

func TestEvaluateLabels(t *testing.T) {
	testCases := map[string]struct {
		Input  string
		Output string
		Labels map[string]string
	}{
		"no labels": {
			Input: `{
				"kind":"Template", "apiVersion":"v1beta1",
				"items": [
					{
						"kind": "Service", "apiVersion": "v1beta3",
						"metadata": {"labels": {"key1": "v1", "key2": "v2"}	}
					}
				]
			}`,
			Output: `{
				"kind":"Template","apiVersion":"v1beta3","metadata":{"creationTimestamp":null},
				"objects":[
					{
						"apiVersion":"v1beta3","kind":"Service","metadata":{
						"labels":{"key1":"v1","key2":"v2"}}
					}
				]
			}`,
		},
		"one different label": {
			Input: `{
				"kind":"Template", "apiVersion":"v1beta1",
				"items": [
					{
						"kind": "Service", "apiVersion": "v1beta3",
						"metadata": {"labels": {"key1": "v1", "key2": "v2"}	}
					}
				]
			}`,
			Output: `{
				"kind":"Template","apiVersion":"v1beta3","metadata":{"creationTimestamp":null},
				"objects":[
					{
						"apiVersion":"v1beta3","kind":"Service","metadata":{
						"labels":{"key1":"v1","key2":"v2","key3":"v3"}}
					}
				],
				"labels":{"key3":"v3"}
			}`,
			Labels: map[string]string{"key3": "v3"},
		},
		"when the root object has labels and no metadata": {
			Input: `{
				"kind":"Template", "apiVersion":"v1beta1",
				"items": [
					{
						"kind": "Service", "apiVersion": "v1beta1",
						"labels": {
							"key1": "v1",
							"key2": "v2"
						}
					}
				]
			}`,
			Output: `{
				"kind":"Template","apiVersion":"v1beta3","metadata":{"creationTimestamp":null},
				"objects":[
					{
						"apiVersion":"v1beta1","kind":"Service",
						"labels":{"key1":"v1","key2":"v2","key3":"v3"}
					}
				],
				"labels":{"key3":"v3"}
			}`,
			Labels: map[string]string{"key3": "v3"},
		},
		"when the root object has labels and metadata": {
			Input: `{
				"kind":"Template", "apiVersion":"v1beta1",
				"items": [
					{
						"kind": "Service", "apiVersion": "v1beta1",
						"metadata": {},
						"labels": {
							"key1": "v1",
							"key2": "v2"
						}
					}
				]
			}`,
			Output: `{
				"kind":"Template","apiVersion":"v1beta3","metadata":{"creationTimestamp":null},
				"objects":[
					{
						"apiVersion":"v1beta1","kind":"Service",
						"labels":{"key1":"v1","key2":"v2"},
						"metadata":{"labels":{"key3":"v3"}}
					}
				],
				"labels":{"key3":"v3"}
			}`,
			Labels: map[string]string{"key3": "v3"},
		},
		"overwrites label": {
			Input: `{
				"kind":"Template", "apiVersion":"v1beta1",
				"items": [
					{
						"kind": "Service", "apiVersion": "v1beta3",
						"metadata": {"labels": {"key1": "v1", "key2": "v2"}	}
					}
				]
			}`,
			Output: `{
				"kind":"Template","apiVersion":"v1beta3","metadata":{"creationTimestamp":null},
				"objects":[
					{
						"apiVersion":"v1beta3","kind":"Service","metadata":{
						"labels":{"key1":"v1","key2":"v3"}}
					}
				],
				"labels":{"key2":"v3"}
			}`,
			Labels: map[string]string{"key2": "v3"},
		},
	}

	for k, testCase := range testCases {
		var template api.Template
		if err := latest.Codec.DecodeInto([]byte(testCase.Input), &template); err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}

		generators := map[string]generator.Generator{
			"expression": generator.NewExpressionValueGenerator(rand.New(rand.NewSource(1337))),
		}
		processor := NewProcessor(generators)

		template.ObjectLabels = testCase.Labels

		// Transform the template config into the result config
		errs := processor.Process(&template)
		if len(errs) > 0 {
			t.Errorf("%s: unexpected error: %v", k, errs)
			continue
		}
		result, err := v1beta3.Codec.Encode(&template)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		expect := testCase.Output
		expect = trailingWhitespace.ReplaceAllString(expect, "")
		if expect != string(result) {
			t.Errorf("%s: unexpected output: %s", k, util.StringDiff(expect, string(result)))
			continue
		}
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
	errs := processor.Process(&template)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	result, err := v1beta3.Codec.Encode(&template)
	if err != nil {
		t.Fatalf("unexpected error during encoding Config: %#v", err)
	}
	expect := `{"kind":"Template","apiVersion":"v1beta3","metadata":{"name":"guestbook-example","creationTimestamp":null,"annotations":{"description":"Example shows how to build a simple multi-tier application using Kubernetes and Docker"}},"objects":[{"apiVersion":"v1beta3","kind":"Route","metadata":{"creationTimestamp":null,"name":"frontend-route"},"spec":{"host":"guestbook.example.com","to":{"kind":"Service","name":"frontend-service"}},"status":{}},{"apiVersion":"v1beta3","kind":"Service","metadata":{"creationTimestamp":null,"name":"frontend-service"},"spec":{"portalIP":"","ports":[{"port":5432,"protocol":"TCP","targetPort":5432}],"selector":{"name":"frontend-service"},"sessionAffinity":"None"},"status":{}},{"apiVersion":"v1beta3","kind":"Service","metadata":{"creationTimestamp":null,"name":"redis-master"},"spec":{"portalIP":"","ports":[{"port":10000,"protocol":"TCP","targetPort":10000}],"selector":{"name":"redis-master"},"sessionAffinity":"None"},"status":{}},{"apiVersion":"v1beta3","kind":"Service","metadata":{"creationTimestamp":null,"name":"redis-slave"},"spec":{"portalIP":"","ports":[{"port":10001,"protocol":"TCP","targetPort":10001}],"selector":{"name":"redis-slave"},"sessionAffinity":"None"},"status":{}},{"apiVersion":"v1beta3","kind":"Pod","metadata":{"creationTimestamp":null,"labels":{"name":"redis-master"},"name":"redis-master"},"spec":{"containers":[{"capabilities":{},"env":[{"name":"REDIS_PASSWORD","value":"P8vxbV4C"}],"image":"dockerfile/redis","imagePullPolicy":"IfNotPresent","name":"master","ports":[{"containerPort":6379,"protocol":"TCP"}],"resources":{},"securityContext":{"capabilities":{},"privileged":false},"terminationMessagePath":"/dev/termination-log"}],"dnsPolicy":"ClusterFirst","restartPolicy":"Always","serviceAccount":""},"status":{}},{"apiVersion":"v1beta3","kind":"ReplicationController","metadata":{"creationTimestamp":null,"labels":{"name":"frontend-service"},"name":"guestbook"},"spec":{"replicas":3,"selector":{"name":"frontend-service"},"template":{"metadata":{"creationTimestamp":null,"labels":{"name":"frontend-service"}},"spec":{"containers":[{"capabilities":{},"env":[{"name":"ADMIN_USERNAME","value":"adminQ3H"},{"name":"ADMIN_PASSWORD","value":"dwNJiJwW"},{"name":"REDIS_PASSWORD","value":"P8vxbV4C"}],"image":"brendanburns/php-redis","imagePullPolicy":"IfNotPresent","name":"php-redis","ports":[{"containerPort":80,"hostPort":8000,"protocol":"TCP"}],"resources":{},"securityContext":{"capabilities":{},"privileged":false},"terminationMessagePath":"/dev/termination-log"}],"dnsPolicy":"ClusterFirst","restartPolicy":"Always","serviceAccount":""}}},"status":{"replicas":0}},{"apiVersion":"v1beta3","kind":"ReplicationController","metadata":{"creationTimestamp":null,"labels":{"name":"redis-slave"},"name":"redis-slave"},"spec":{"replicas":2,"selector":{"name":"redis-slave"},"template":{"metadata":{"creationTimestamp":null,"labels":{"name":"redis-slave"}},"spec":{"containers":[{"capabilities":{},"env":[{"name":"REDIS_PASSWORD","value":"P8vxbV4C"}],"image":"brendanburns/redis-slave","imagePullPolicy":"IfNotPresent","name":"slave","ports":[{"containerPort":6379,"hostPort":6380,"protocol":"TCP"}],"resources":{},"securityContext":{"capabilities":{},"privileged":false},"terminationMessagePath":"/dev/termination-log"}],"dnsPolicy":"ClusterFirst","restartPolicy":"Always","serviceAccount":""}}},"status":{"replicas":0}}],"parameters":[{"name":"ADMIN_USERNAME","description":"Guestbook administrator username","value":"adminQ3H","generate":"expression","from":"admin[A-Z0-9]{3}"},{"name":"ADMIN_PASSWORD","description":"Guestbook administrator password","value":"dwNJiJwW","generate":"expression","from":"[a-zA-Z0-9]{8}"},{"name":"REDIS_PASSWORD","description":"Redis password","value":"P8vxbV4C","generate":"expression","from":"[a-zA-Z0-9]{8}"},{"name":"SLAVE_SERVICE_NAME","description":"Slave Service name","value":"redis-slave"},{"name":"CUSTOM_PARAM1","value":"1"}]}`

	expect = strings.Replace(expect, "\n", "", -1)
	if string(result) != expect {
		t.Errorf("unexpected output: %s", util.StringDiff(expect, string(result)))
	}
}
