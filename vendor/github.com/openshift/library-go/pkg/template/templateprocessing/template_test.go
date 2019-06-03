package templateprocessing

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/apitesting"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/validation/field"

	appsv1 "github.com/openshift/api/apps/v1"
	templatev1 "github.com/openshift/api/template/v1"
	"github.com/openshift/library-go/pkg/template/generator"
)

var codecFactory = serializer.CodecFactory{}

func init() {
	_, codecFactory = apitesting.SchemeForOrDie(templatev1.Install)
}

func makeParameter(name, value, generate string, required bool) templatev1.Parameter {
	return templatev1.Parameter{
		Name:     name,
		Value:    value,
		Generate: generate,
		Required: required,
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

type NoStringGenerator struct {
}

func (g NoStringGenerator) GenerateValue(expression string) (interface{}, error) {
	return NoStringGenerator{}, nil
}

type EmptyGenerator struct {
}

func (g EmptyGenerator) GenerateValue(expression string) (interface{}, error) {
	return "", nil
}

func TestParameterGenerators(t *testing.T) {
	tests := []struct {
		parameter  templatev1.Parameter
		generators map[string]generator.Generator
		shouldPass bool
		expected   templatev1.Parameter
		errType    field.ErrorType
		fieldPath  string
	}{
		{ // Empty generator, should pass
			makeParameter("PARAM-pass-empty-gen", "X", "", false),
			map[string]generator.Generator{},
			true,
			makeParameter("PARAM-pass-empty-gen", "X", "", false),
			"",
			"",
		},
		{ // Foo generator, should pass
			makeParameter("PARAM-pass-foo-gen", "", "foo", false),
			map[string]generator.Generator{"foo": FooGenerator{}},
			true,
			makeParameter("PARAM-pass-foo-gen", "foo", "", false),
			"",
			"",
		},
		{ // Foo generator, should fail
			makeParameter("PARAM-fail-foo-gen", "", "foo", false),
			map[string]generator.Generator{},
			false,
			makeParameter("PARAM-fail-foo-gen", "foo", "", false),
			field.ErrorTypeInvalid,
			"template.parameters[0]",
		},
		{ // No str generator, should fail
			makeParameter("PARAM-fail-nostr-gen", "", "foo", false),
			map[string]generator.Generator{"foo": NoStringGenerator{}},
			false,
			makeParameter("PARAM-fail-nostr-gen", "foo", "", false),
			field.ErrorTypeInvalid,
			"template.parameters[0]",
		},
		{ // Invalid generator, should fail
			makeParameter("PARAM-fail-inv-gen", "", "invalid", false),
			map[string]generator.Generator{"invalid": nil},
			false,
			makeParameter("PARAM-fail-inv-gen", "", "invalid", false),
			field.ErrorTypeInvalid,
			"template.parameters[0]",
		},
		{ // Error generator, should fail
			makeParameter("PARAM-fail-err-gen", "", "error", false),
			map[string]generator.Generator{"error": ErrorGenerator{}},
			false,
			makeParameter("PARAM-fail-err-gen", "", "error", false),
			field.ErrorTypeInvalid,
			"template.parameters[0]",
		},
		{ // Error required parameter, no value, should fail
			makeParameter("PARAM-fail-no-val", "", "", true),
			map[string]generator.Generator{"error": ErrorGenerator{}},
			false,
			makeParameter("PARAM-fail-no-val", "", "", true),
			field.ErrorTypeRequired,
			"template.parameters[0]",
		},
		{ // Error required parameter, no value from generator, should fail
			makeParameter("PARAM-fail-no-val-from-gen", "", "empty", true),
			map[string]generator.Generator{"empty": EmptyGenerator{}},
			false,
			makeParameter("PARAM-fail-no-val-from-gen", "", "empty", true),
			field.ErrorTypeRequired,
			"template.parameters[0]",
		},
	}

	for i, test := range tests {
		processor := NewProcessor(test.generators)
		template := templatev1.Template{Parameters: []templatev1.Parameter{test.parameter}}
		errs := processor.GenerateParameterValues(&template)
		if errs != nil && test.shouldPass {
			t.Errorf("test[%v]: Unexpected error %v", i, errs)
		}
		if errs == nil && !test.shouldPass {
			t.Errorf("test[%v]: Expected error", i)
		}
		if errs != nil {
			if test.errType != errs[0].Type {
				t.Errorf("test[%v]: Unexpected error type: Expected: %s, got %s", i, test.errType, errs[0].Type)
			}
			if test.fieldPath != errs[0].Field {
				t.Errorf("test[%v]: Unexpected error type: Expected: %s, got %s", i, test.fieldPath, errs[0].Field)
			}
			continue
		}
		actual := template.Parameters[0]
		if actual.Value != test.expected.Value {
			t.Errorf("test[%v]: Unexpected value: Expected: %#v, got: %#v", i, test.expected.Value, test.parameter.Value)
		}
	}
}

func TestProcessValue(t *testing.T) {
	var template templatev1.Template
	if err := runtime.DecodeInto(codecFactory.UniversalDecoder(), []byte(`{
		"kind":"Template", "apiVersion":"template.openshift.io/v1",
		"objects": [
			{
				"kind": "Service", "apiVersion": "v${VALUE}",
				"metadata": {
					"labels": {
						"i1": "${{INT_1}}",
						"invalidjsonmap": "${{INVALID_JSON_MAP}}",
						"invalidjsonarray": "${{INVALID_JSON_ARRAY}}",
						"key1": "${VALUE}",
						"key2": "$${VALUE}",
						"quoted_string": "${{STRING_1}}",
						"s1_s1": "${STRING_1}_${STRING_1}",
						"s1_s2": "${STRING_1}_${STRING_2}",
						"untouched": "a${{INT_1}}",
						"untouched2": "${{INT_1}}a",
						"untouched3": "${{INVALID_PARAMETER}}",
						"untouched4": "${{INVALID PARAMETER}}",
						"validjsonmap": "${{VALID_JSON_MAP}}",
						"validjsonarray": "${{VALID_JSON_ARRAY}}"

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
	addParameter(&template, makeParameter("VALUE", "1", "", false))
	addParameter(&template, makeParameter("STRING_1", "string1", "", false))
	addParameter(&template, makeParameter("STRING_2", "string2", "", false))
	addParameter(&template, makeParameter("INT_1", "1", "", false))
	addParameter(&template, makeParameter("VALID_JSON_MAP", "{\"key\":\"value\"}", "", false))
	addParameter(&template, makeParameter("INVALID_JSON_MAP", "{\"key\":\"value\"", "", false))
	addParameter(&template, makeParameter("VALID_JSON_ARRAY", "[\"key\",\"value\"]", "", false))
	addParameter(&template, makeParameter("INVALID_JSON_ARRAY", "[\"key\":\"value\"", "", false))

	// Transform the template config into the result config
	errs := processor.Process(&template)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	result, err := runtime.Encode(codecFactory.LegacyCodec(templatev1.GroupVersion), &template)
	if err != nil {
		t.Fatalf("unexpected error during encoding Config: %#v", err)
	}
	expect := `{"kind":"Template","apiVersion":"template.openshift.io/v1","metadata":{"creationTimestamp":null},"objects":[{"apiVersion":"v1","kind":"Service","metadata":{"labels":{"i1":1,"invalidjsonarray":"[\"key\":\"value\"","invalidjsonmap":"{\"key\":\"value\"","key1":"1","key2":"$1","quoted_string":"string1","s1_s1":"string1_string1","s1_s2":"string1_string2","untouched":"a${{INT_1}}","untouched2":"${{INT_1}}a","untouched3":"${{INVALID_PARAMETER}}","untouched4":"${{INVALID PARAMETER}}","validjsonarray":["key","value"],"validjsonmap":{"key":"value"}}}}],"parameters":[{"name":"VALUE","value":"1"},{"name":"STRING_1","value":"string1"},{"name":"STRING_2","value":"string2"},{"name":"INT_1","value":"1"},{"name":"VALID_JSON_MAP","value":"{\"key\":\"value\"}"},{"name":"INVALID_JSON_MAP","value":"{\"key\":\"value\""},{"name":"VALID_JSON_ARRAY","value":"[\"key\",\"value\"]"},{"name":"INVALID_JSON_ARRAY","value":"[\"key\":\"value\""}]}`
	stringResult := strings.TrimSpace(string(result))
	if expect != stringResult {
		//t.Errorf("unexpected output, expected: \n%s\nGot:\n%s\n", expect, stringResult)
		t.Errorf("unexpected output: %s", diff.StringDiff(expect, stringResult))
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
				"kind":"Template", "apiVersion":"template.openshift.io/v1",
				"objects": [
					{
						"kind": "Service", "apiVersion": "v1",
						"metadata": {"labels": {"key1": "v1", "key2": "v2"}	}
					}
				]
			}`,
			Output: `{
				"kind":"Template","apiVersion":"template.openshift.io/v1","metadata":{"creationTimestamp":null},
				"objects":[
					{
						"apiVersion":"v1","kind":"Service","metadata":{
						"labels":{"key1":"v1","key2":"v2"}}
					}
				]
			}`,
		},
		"one different label": {
			Input: `{
				"kind":"Template", "apiVersion":"template.openshift.io/v1",
				"objects": [
					{
						"kind": "Service", "apiVersion": "v1",
						"metadata": {"labels": {"key1": "v1", "key2": "v2"}	}
					}
				]
			}`,
			Output: `{
				"kind":"Template","apiVersion":"template.openshift.io/v1","metadata":{"creationTimestamp":null},
				"objects":[
					{
						"apiVersion":"v1","kind":"Service","metadata":{
						"labels":{"key1":"v1","key2":"v2","key3":"v3"}}
					}
				],
				"labels":{"key3":"v3"}
			}`,
			Labels: map[string]string{"key3": "v3"},
		},
		"when the root object has labels and metadata": {
			Input: `{
				"kind":"Template", "apiVersion":"template.openshift.io/v1",
				"objects": [
					{
						"kind": "Service", "apiVersion": "v1",
						"metadata": {},
						"labels": {
							"key1": "v1",
							"key2": "v2"
						}
					}
				]
			}`,
			Output: `{
				"kind":"Template","apiVersion":"template.openshift.io/v1","metadata":{"creationTimestamp":null},
				"objects":[
					{
						"apiVersion":"v1","kind":"Service",
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
				"kind":"Template", "apiVersion":"template.openshift.io/v1",
				"objects": [
					{
						"kind": "Service", "apiVersion": "v1",
						"metadata": {"labels": {"key1": "v1", "key2": "v2"}	}
					}
				]
			}`,
			Output: `{
				"kind":"Template","apiVersion":"template.openshift.io/v1","metadata":{"creationTimestamp":null},
				"objects":[
					{
						"apiVersion":"v1","kind":"Service","metadata":{
						"labels":{"key1":"v1","key2":"v3"}}
					}
				],
				"labels":{"key2":"v3"}
			}`,
			Labels: map[string]string{"key2": "v3"},
		},
		"parameterised labels": {
			Input: `{
				"kind":"Template", "apiVersion":"template.openshift.io/v1",
				"objects": [
					{
						"kind": "Service", "apiVersion": "v1",
						"metadata": {"labels": {"key1": "v1", "key2": "v2"}}
					}
				],
				"parameters": [
					{
						"name": "KEY",
						"value": "key"
					},
					{
						"name": "VALUE",
						"value": "value"
					}
				]
			}`,
			Output: `{
				"kind":"Template","apiVersion":"template.openshift.io/v1","metadata":{"creationTimestamp":null},
				"objects":[
					{
						"apiVersion":"v1","kind":"Service","metadata":{
						"labels":{"key":"value","key1":"v1","key2":"v2"}}
					}
				],
				"parameters":[
					{
						"name":"KEY",
						"value":"key"
					},
					{
						"name":"VALUE",
						"value":"value"
					}
				],
				"labels":{"key":"value"}
			}`,
			Labels: map[string]string{"${KEY}": "${VALUE}"},
		},
	}

	for k, testCase := range testCases {
		var template templatev1.Template
		if err := runtime.DecodeInto(codecFactory.UniversalDecoder(), []byte(testCase.Input), &template); err != nil {
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
		result, err := runtime.Encode(codecFactory.LegacyCodec(templatev1.GroupVersion), &template)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		expect := testCase.Output
		expect = trailingWhitespace.ReplaceAllString(expect, "")
		stringResult := strings.TrimSpace(string(result))
		if expect != stringResult {
			t.Errorf("%s: unexpected output: %s", k, diff.StringDiff(expect, stringResult))
			continue
		}
	}
}

func TestProcessTemplateParameters(t *testing.T) {
	var template, expectedTemplate templatev1.Template
	jsonData, _ := ioutil.ReadFile("testdata/guestbook.json")
	if err := runtime.DecodeInto(codecFactory.UniversalDecoder(), jsonData, &template); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedData, _ := ioutil.ReadFile("testdata/guestbook_list.json")
	if err := runtime.DecodeInto(codecFactory.UniversalDecoder(), expectedData, &expectedTemplate); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	generators := map[string]generator.Generator{
		"expression": generator.NewExpressionValueGenerator(rand.New(rand.NewSource(1337))),
	}
	processor := NewProcessor(generators)

	// Define custom parameter for the transformation:
	addParameter(&template, makeParameter("CUSTOM_PARAM1", "1", "", false))

	// Transform the template config into the result config
	errs := processor.Process(&template)
	if len(errs) > 0 {
		t.Fatalf("unexpected error: %v", errs)
	}
	result, err := runtime.Encode(codecFactory.LegacyCodec(templatev1.GroupVersion), &template)
	if err != nil {
		t.Fatalf("unexpected error during encoding Config: %#v", err)
	}
	exp, _ := runtime.Encode(codecFactory.LegacyCodec(templatev1.GroupVersion), &expectedTemplate)

	if string(result) != string(exp) {
		t.Errorf("unexpected output: %s", diff.StringDiff(string(exp), string(result)))
	}
}

// addParameter adds new custom parameter to the Template. It overrides
// the existing parameter, if already defined.
func addParameter(t *templatev1.Template, param templatev1.Parameter) {
	if existing := GetParameterByName(t, param.Name); existing != nil {
		*existing = param
	} else {
		t.Parameters = append(t.Parameters, param)
	}
}

func TestAddConfigLabels(t *testing.T) {
	var nilLabels map[string]string

	testCases := []struct {
		obj            runtime.Object
		addLabels      map[string]string
		err            bool
		expectedLabels map[string]string
	}{
		{ // [0] Test nil + nil => nil
			obj:            &corev1.Pod{},
			addLabels:      nilLabels,
			err:            false,
			expectedLabels: nilLabels,
		},
		{ // [1] Test nil + empty labels => empty labels
			obj:            &corev1.Pod{},
			addLabels:      map[string]string{},
			err:            false,
			expectedLabels: map[string]string{},
		},
		{ // [2] Test obj.Labels + nil => obj.Labels
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
			},
			addLabels:      nilLabels,
			err:            false,
			expectedLabels: map[string]string{"foo": "bar"},
		},
		{ // [3] Test obj.Labels + empty labels => obj.Labels
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
			},
			addLabels:      map[string]string{},
			err:            false,
			expectedLabels: map[string]string{"foo": "bar"},
		},
		{ // [4] Test nil + addLabels => addLabels
			obj:            &corev1.Pod{},
			addLabels:      map[string]string{"foo": "bar"},
			err:            false,
			expectedLabels: map[string]string{"foo": "bar"},
		},
		{ // [5] Test obj.labels + addLabels => expectedLabels
			obj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"baz": ""}},
			},
			addLabels:      map[string]string{"foo": "bar"},
			err:            false,
			expectedLabels: map[string]string{"foo": "bar", "baz": ""},
		},
		{ // [6] Test conflicting keys with the same value
			obj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "same value"}},
			},
			addLabels:      map[string]string{"foo": "same value"},
			err:            false,
			expectedLabels: map[string]string{"foo": "same value"},
		},
		{ // [7] Test conflicting keys with a different value
			obj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "first value"}},
			},
			addLabels:      map[string]string{"foo": "second value"},
			err:            false,
			expectedLabels: map[string]string{"foo": "second value"},
		},
		{ // [8] Test conflicting keys with the same value in ReplicationController nested labels
			obj: &corev1.ReplicationController{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"foo": "same value"},
				},
				Spec: corev1.ReplicationControllerSpec{
					Template: &corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{},
						},
					},
				},
			},
			addLabels:      map[string]string{"foo": "same value"},
			err:            false,
			expectedLabels: map[string]string{"foo": "same value"},
		},
		{ // [9] Test adding labels to a DeploymentConfig object
			obj: &appsv1.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"foo": "first value"},
				},
				Spec: appsv1.DeploymentConfigSpec{
					Template: &corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"foo": "first value"},
						},
					},
				},
			},
			addLabels:      map[string]string{"bar": "second value"},
			err:            false,
			expectedLabels: map[string]string{"foo": "first value", "bar": "second value"},
		},
	}

	for i, test := range testCases {
		err := addObjectLabels(test.obj, test.addLabels)
		if err != nil && !test.err {
			t.Errorf("Unexpected error while setting labels on testCase[%v]: %v.", i, err)
		} else if err == nil && test.err {
			t.Errorf("Unexpected non-error while setting labels on testCase[%v].", i)
		}

		accessor, err := meta.Accessor(test.obj)
		if err != nil {
			t.Error(err)
		}
		metaLabels := accessor.GetLabels()
		if e, a := test.expectedLabels, metaLabels; !reflect.DeepEqual(e, a) {
			t.Errorf("Unexpected labels on testCase[%v]. Expected: %#v, got: %#v.", i, e, a)
		}

		// must not add any new nested labels
		switch objType := test.obj.(type) {
		case *corev1.ReplicationController:
			if e, a := map[string]string{}, objType.Spec.Template.Labels; !reflect.DeepEqual(e, a) {
				t.Errorf("Unexpected labels on testCase[%v]. Expected: %#v, got: %#v.", i, e, a)
			}
		case *appsv1.DeploymentConfig:
			if e, a := test.expectedLabels, objType.Spec.Template.Labels; !reflect.DeepEqual(e, a) {
				t.Errorf("Unexpected labels on testCase[%v]. Expected: %#v, got: %#v.", i, e, a)
			}
		}
	}
}
