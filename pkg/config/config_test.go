package config

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	clientapi "github.com/openshift/origin/pkg/cmd/client/api"
	"github.com/openshift/origin/pkg/config/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

func TestApplyInvalidConfig(t *testing.T) {
	clients := clientapi.ClientMappings{
		"InvalidClientMapping": {"InvalidClientResource", nil, nil},
	}
	invalidConfigs := []string{
		`{}`,
		`{ "foo": "bar" }`,
		`{ "items": null }`,
		`{ "items": "bar" }`,
		`{ "items": [ null ] }`,
		`{ "items": [ { "foo": "bar" } ] }`,
		`{ "items": [ { "kind": "", "apiVersion": "v1beta1" } ] }`,
		`{ "items": [ { "kind": "UnknownResource", "apiVersion": "v1beta1" } ] }`,
		`{ "items": [ { "kind": "InvalidClientResource", "apiVersion": "v1beta1" } ] }`,
	}
	for i, invalidConfig := range invalidConfigs {
		result, _ := Apply(kapi.NamespaceDefault, []byte(invalidConfig), clients)

		if result != nil {
			t.Errorf("Expected error while applying invalid Config '%v'", invalidConfig[i])
		}
		for _, itemResult := range result {
			if itemResult.Error == nil {
				t.Errorf("Expected error while applying invalid Config '%v'", invalidConfig[i])
			}
			if _, ok := itemResult.Error.(kclient.APIStatus); ok {
				t.Errorf("Unexpected conversion of %T into kclient.APIStatus", itemResult.Error)
			}
		}
	}
}

type FakeResource struct {
	kapi.TypeMeta `json:",inline" yaml:",inline"`
}

func (*FakeResource) IsAnAPIObject() {}

func TestApplySendsData(t *testing.T) {
	fakeScheme := runtime.NewScheme()
	// TODO: The below should work with "FakeResource" name instead.
	fakeScheme.AddKnownTypeWithName("", "", &FakeResource{})
	fakeScheme.AddKnownTypeWithName("v1beta1", "", &FakeResource{})
	fakeCodec := runtime.CodecFor(fakeScheme, "v1beta1")

	received := make(chan bool, 1)
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- true
		if r.RequestURI != "/api/v1beta1/FakeMapping?namespace="+kapi.NamespaceDefault {
			t.Errorf("Unexpected RESTClient RequestURI. Expected: %v, got: %v.", "/api/v1beta1/FakeMapping", r.RequestURI)
		}
	}))

	uri, _ := url.Parse(fakeServer.URL + "/api/v1beta1")
	fakeClient := kclient.NewRESTClient(uri, fakeCodec)
	clients := clientapi.ClientMappings{
		"FakeMapping": {"FakeResource", fakeClient, fakeCodec},
	}
	config := `{ "apiVersion": "v1beta1", "items": [ { "kind": "FakeResource", "apiVersion": "v1beta1", "name": "FakeID" } ] }`
	result, err := Apply(kapi.NamespaceDefault, []byte(config), clients)

	if err != nil || result == nil {
		t.Errorf("Unexpected error while applying valid Config '%v': %v", config, err)
	}
	for _, itemResult := range result {
		if itemResult.Error == nil {
			continue
		}

		if _, ok := itemResult.Error.(kclient.APIStatus); ok {
			t.Errorf("Unexpected error while applying valid Config '%v': %v", config, itemResult.Error)
		} else {
			t.Errorf("Cannot convert %T into kclient.APIStatus", itemResult.Error)
		}
	}

	<-received
}

func TestGetClientAndPath(t *testing.T) {
	kubeClient, _ := kclient.New(&kclient.Config{Host: "127.0.0.1"})
	testClientMappings := clientapi.ClientMappings{
		"pods":     {"Pod", kubeClient.RESTClient, klatest.Codec},
		"services": {"Service", kubeClient.RESTClient, klatest.Codec},
	}
	client, path, _ := getClientAndPath("Service", testClientMappings)
	if client != kubeClient.RESTClient {
		t.Errorf("Failed to get client for Service")
	}
	if path != "services" {
		t.Errorf("Failed to get path for Service")
	}
}

func ExampleApply() {
	kubeClient, _ := kclient.New(&kclient.Config{Host: "127.0.0.1"})
	testClientMappings := clientapi.ClientMappings{
		"pods":     {"Pod", kubeClient.RESTClient, klatest.Codec},
		"services": {"Service", kubeClient.RESTClient, klatest.Codec},
	}
	data, _ := ioutil.ReadFile("config_test.json")
	Apply(kapi.NamespaceDefault, data, testClientMappings)
}

type FakeLabelsResource struct {
	kapi.TypeMeta `json:",inline" yaml:",inline"`
	Labels        map[string]string `json:"labels" yaml:"labels"`
}

func (*FakeLabelsResource) IsAnAPIObject() {}

func TestAddConfigLabels(t *testing.T) {
	testCases := []struct {
		resource       runtime.Object
		addLabels      map[string]string
		shouldPass     bool
		expectedLabels map[string]string
	}{
		{ // Test empty labels
			&kapi.Pod{},
			map[string]string{},
			true,
			map[string]string{},
		},
		{ // Test resource labels + 0 => expected labels
			&kapi.Pod{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"foo": "bar"}}},
			map[string]string{},
			true,
			map[string]string{"foo": "bar"},
		},
		{ // Test 0 + addLabels => expected labels
			&kapi.Pod{},
			map[string]string{"foo": "bar"},
			true,
			map[string]string{"foo": "bar"},
		},
		{ // Test resource labels + addLabels => expected labels
			&kapi.Service{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"baz": ""}}},
			map[string]string{"foo": "bar"},
			true,
			map[string]string{"foo": "bar", "baz": ""},
		},
		{ // Test conflicting keys with the same value
			&kapi.Service{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"foo": "same value"}}},
			map[string]string{"foo": "same value"},
			true,
			map[string]string{"foo": "same value"},
		},
		{ // Test conflicting keys with a different value
			&kapi.Service{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"foo": "first value"}}},
			map[string]string{"foo": "second value"},
			false,
			map[string]string{"foo": "first value"},
		},
		{ // Test conflicting keys with the same value in the nested ReplicationController labels
			&kapi.ReplicationController{
				ObjectMeta: kapi.ObjectMeta{
					Labels: map[string]string{"foo": "same value"},
				},
				DesiredState: kapi.ReplicationControllerState{
					PodTemplate: kapi.PodTemplate{
						Labels: map[string]string{"foo": "same value"},
					},
				},
			},
			map[string]string{"foo": "same value"},
			true,
			map[string]string{"foo": "same value"},
		},
		{ // Test conflicting keys with a different value in the nested ReplicationController labels
			&kapi.ReplicationController{
				ObjectMeta: kapi.ObjectMeta{
					Labels: map[string]string{"foo": "first value"},
				},
				DesiredState: kapi.ReplicationControllerState{
					PodTemplate: kapi.PodTemplate{
						Labels: map[string]string{"foo": "first value"},
					},
				},
			},
			map[string]string{"foo": "second value"},
			false,
			map[string]string{"foo": "first value"},
		},
		{ // Test merging into deployment object
			&deployapi.Deployment{
				Labels: map[string]string{"foo": "first value"},
				ControllerTemplate: kapi.ReplicationControllerState{
					PodTemplate: kapi.PodTemplate{
						Labels: map[string]string{"foo": "first value"},
					},
				},
			},
			map[string]string{"bar": "second value"},
			true,
			map[string]string{"foo": "first value", "bar": "second value"},
		},
		{ // Test merging into DeploymentConfig
			&deployapi.DeploymentConfig{
				Labels: map[string]string{"foo": "first value"},
				Template: deployapi.DeploymentTemplate{
					ControllerTemplate: kapi.ReplicationControllerState{
						PodTemplate: kapi.PodTemplate{
							Labels: map[string]string{"foo": "first value"},
						},
					},
				},
			},
			map[string]string{"bar": "second value"},
			true,
			map[string]string{"foo": "first value", "bar": "second value"},
		},
		{ // Test unknown Generic Object with Labels field
			&FakeLabelsResource{Labels: map[string]string{"baz": ""}},
			map[string]string{"foo": "bar"},
			true,
			map[string]string{"foo": "bar", "baz": ""},
		},
	}

	for i, test := range testCases {
		items := []runtime.EmbeddedObject{{Object: test.resource}}
		cfg := api.Config{Items: items}

		err := AddConfigLabels(&cfg, test.addLabels)
		if err != nil && test.shouldPass {
			t.Errorf("Unexpected error while setting labels on testCase[%v].", i)
		}
		if err == nil && !test.shouldPass {
			t.Errorf("Unexpected non-error while setting labels on testCase[%v].", i)
		}

		obj := reflect.ValueOf(cfg.Items[0].Object)
		if obj.Kind() == reflect.Interface || obj.Kind() == reflect.Ptr {
			obj = obj.Elem()
		}

		// Test Item[i].Labels.
		rootLabels := obj.FieldByName("Labels").Interface().(map[string]string)
		if !reflect.DeepEqual(rootLabels, test.expectedLabels) {
			t.Errorf("Unexpected root labels on testCase[%v]. Expected: %v, got: %v.", i, test.expectedLabels, rootLabels)
		}

		// Test ReplicationController's nested labels.
		if obj.Type().Name() == "ReplicationController" {
			// Test Items[i].DesiredState.PodTemplate.Labels.
			nestedLabels := obj.FieldByName("DesiredState").FieldByName("PodTemplate").FieldByName("Labels").Interface().(map[string]string)
			if !reflect.DeepEqual(nestedLabels, test.expectedLabels) {
				t.Errorf("Unexpected nested labels on testCase[%v]. Expected: %v, got: %v.", i, test.expectedLabels, nestedLabels)
			}
		}
		// Test Deployment's nested labels.
		if obj.Type().Name() == "Deployment" {
			// Test Items[i].ControllerTemplate.PodTemplate.Labels.
			nestedLabels := obj.FieldByName("ControllerTemplate").FieldByName("PodTemplate").FieldByName("Labels").Interface().(map[string]string)
			if !reflect.DeepEqual(nestedLabels, test.expectedLabels) {
				t.Errorf("Unexpected nested labels on testCase[%v]. Expected: %v, got: %v.", i, test.expectedLabels, nestedLabels)
			}
		}

		// Test DeploymentConfig's nested labels.
		if obj.Type().Name() == "DeploymentConfig" {
			// Test Items[i].ControllerTemplate.PodTemplate.Labels.
			nestedLabels := obj.FieldByName("Template").FieldByName("ControllerTemplate").FieldByName("PodTemplate").FieldByName("Labels").Interface().(map[string]string)
			if !reflect.DeepEqual(nestedLabels, test.expectedLabels) {
				t.Errorf("Unexpected nested labels on testCase[%v]. Expected: %v, got: %v.", i, test.expectedLabels, nestedLabels)
			}
		}
	}
}

func TestMergeMaps(t *testing.T) {
	testCases := []struct {
		dst        interface{}
		src        interface{}
		flags      int
		shouldPass bool
		expected   interface{}
	}{
		{ // Test empty maps
			map[int]int{},
			map[int]int{},
			0,
			true,
			map[int]int{},
		},
		{ // Test dst + src => expected
			map[int]string{1: "foo"},
			map[int]string{2: "bar"},
			0,
			true,
			map[int]string{1: "foo", 2: "bar"},
		},
		{ // Test dst + src => expected, do not overwrite dst
			map[string]string{"foo": "bar"},
			map[string]string{"foo": ""},
			0,
			true,
			map[string]string{"foo": "bar"},
		},
		{ // Test dst + src => expected, overwrite dst
			map[string]string{"foo": "bar"},
			map[string]string{"foo": ""},
			OverwriteExistingDstKey,
			true,
			map[string]string{"foo": ""},
		},
		{ // Test dst + src => expected, error on existing key value
			map[string]string{"foo": "bar"},
			map[string]string{"foo": "bar"},
			ErrorOnExistingDstKey | OverwriteExistingDstKey,
			false,
			map[string]string{"foo": "bar"},
		},
		{ // Test dst + src => expected, do not error on same key value
			map[string]string{"foo": "bar"},
			map[string]string{"foo": "bar"},
			ErrorOnDifferentDstKeyValue | OverwriteExistingDstKey,
			true,
			map[string]string{"foo": "bar"},
		},
		{ // Test dst + src => expected, error on different key value
			map[string]string{"foo": "bar"},
			map[string]string{"foo": ""},
			ErrorOnDifferentDstKeyValue | OverwriteExistingDstKey,
			false,
			map[string]string{"foo": "bar"},
		},
	}

	for i, test := range testCases {
		err := mergeMaps(test.dst, test.src, test.flags)
		if err != nil && test.shouldPass {
			t.Errorf("Unexpected error while merging maps on testCase[%v].", i)
		}
		if err == nil && !test.shouldPass {
			t.Errorf("Unexpected non-error while merging maps on testCase[%v].", i)
		}
		if !reflect.DeepEqual(test.dst, test.expected) {
			t.Errorf("Unexpected map on testCase[%v]. Expected: %v, got: %v.", i, test.expected, test.dst)
		}
	}
}
