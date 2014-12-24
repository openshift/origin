package config

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	kmeta "github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	clientapi "github.com/openshift/origin/pkg/cmd/client/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

func TestApplyInvalidConfig(t *testing.T) {
	clients := clientapi.ClientMappings{
		"InvalidClientMapping": {"InvalidClientResource", nil, nil},
	}
	clientFunc := func(m *kmeta.RESTMapping) (*kubectl.RESTHelper, error) {
		mapping, ok := clients[m.Resource]
		if !ok {
			return nil, fmt.Errorf("Unable to provide REST client for %v", m.Resource)
		}
		return kubectl.NewRESTHelper(mapping.Client, m), nil
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
		result, err := Apply(kapi.NamespaceDefault, []byte(invalidConfig), clientFunc)

		if i <= 3 && err == nil {
			t.Errorf("Expected error while applying invalid Config '%v', result: %v", invalidConfigs[i], result)
		}

		for _, itemResult := range result {
			if len(itemResult.Errors) > 0 {
				t.Errorf("Expected error while applying invalid Config '%v'", invalidConfigs[i])
			}
		}
	}
}

func TestApplySendsData(t *testing.T) {
	received := make(chan bool, 1)
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- true
		if r.RequestURI != "/api/v1beta1/pods?namespace="+kapi.NamespaceDefault {
			t.Errorf("Unexpected RESTClient RequestURI. Expected: %v, got: %v.", "/api/v1beta1/pods", r.RequestURI)
		}
	}))

	uri, _ := url.Parse(fakeServer.URL + "/api/v1beta1")
	fakeClient := kclient.NewRESTClient(uri, kapi.Codec, false)
	clients := clientapi.ClientMappings{
		"pods": {"Pod", fakeClient, kapi.Codec},
	}
	clientFunc := func(m *kmeta.RESTMapping) (*kubectl.RESTHelper, error) {
		mapping, ok := clients[m.Resource]
		if !ok {
			return nil, fmt.Errorf("Unable to provide REST client for %v", m.Resource)
		}
		return kubectl.NewRESTHelper(mapping.Client, m), nil
	}
	config := `{ "apiVersion": "v1beta1", "kind": "Config", "metadata" : { "name": "test-config" }, "items": [ { "kind": "Pod", "apiVersion": "v1beta1", "metadata": { "name": "FakePod" } } ] }`
	result, err := Apply(kapi.NamespaceDefault, []byte(config), clientFunc)

	if err != nil || result == nil {
		t.Errorf("Unexpected error while applying valid Config '%v', result: %v, error: %v", config, result, err)
	}

	for _, itemResult := range result {
		if len(itemResult.Errors) > 0 {
			t.Errorf("Unexpected error while applying valid Config '%v': %+v", config, itemResult.Errors)
		}
	}

	// <-received
}

func ExampleApply() {
	kubeClient, _ := kclient.New(&kclient.Config{Host: "127.0.0.1"})
	testClientMappings := clientapi.ClientMappings{
		"pods":     {"Pod", kubeClient.RESTClient, klatest.Codec},
		"services": {"Service", kubeClient.RESTClient, klatest.Codec},
	}
	clientFunc := func(m *kmeta.RESTMapping) (*kubectl.RESTHelper, error) {
		mapping, ok := testClientMappings[m.Resource]
		if !ok {
			return nil, fmt.Errorf("Unable to provide REST client for %v", m.Resource)
		}
		return kubectl.NewRESTHelper(mapping.Client, m), nil
	}
	data, _ := ioutil.ReadFile("../../examples/sample-app/docker-registry-config.json")
	Apply(kapi.NamespaceDefault, data, clientFunc)
}

type FakeLabelsResource struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

func (*FakeLabelsResource) IsAnAPIObject() {}

func TestAddConfigLabels(t *testing.T) {
	var nilLabels map[string]string

	testCases := []struct {
		obj            runtime.Object
		addLabels      map[string]string
		err            bool
		expectedLabels map[string]string
	}{
		{ // [0] Test nil + nil => nil
			obj:            &kapi.Pod{},
			addLabels:      nilLabels,
			err:            false,
			expectedLabels: nilLabels,
		},
		{ // [1] Test nil + empty labels => empty labels
			obj:            &kapi.Pod{},
			addLabels:      map[string]string{},
			err:            false,
			expectedLabels: map[string]string{},
		},
		{ // [2] Test obj.Labels + nil => obj.Labels
			obj: &kapi.Pod{
				ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
			},
			addLabels:      nilLabels,
			err:            false,
			expectedLabels: map[string]string{"foo": "bar"},
		},
		{ // [3] Test obj.Labels + empty labels => obj.Labels
			obj: &kapi.Pod{
				ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
			},
			addLabels:      map[string]string{},
			err:            false,
			expectedLabels: map[string]string{"foo": "bar"},
		},
		{ // [4] Test nil + addLabels => addLabels
			obj:            &kapi.Pod{},
			addLabels:      map[string]string{"foo": "bar"},
			err:            false,
			expectedLabels: map[string]string{"foo": "bar"},
		},
		{ // [5] Test obj.labels + addLabels => expectedLabels
			obj: &kapi.Service{
				ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"baz": ""}},
			},
			addLabels:      map[string]string{"foo": "bar"},
			err:            false,
			expectedLabels: map[string]string{"foo": "bar", "baz": ""},
		},
		{ // [6] Test conflicting keys with the same value
			obj: &kapi.Service{
				ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"foo": "same value"}},
			},
			addLabels:      map[string]string{"foo": "same value"},
			err:            false,
			expectedLabels: map[string]string{"foo": "same value"},
		},
		{ // [7] Test conflicting keys with a different value
			obj: &kapi.Service{
				ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"foo": "first value"}},
			},
			addLabels:      map[string]string{"foo": "second value"},
			err:            true,
			expectedLabels: map[string]string{"foo": "first value"},
		},
		{ // [8] Test conflicting keys with the same value in ReplicationController nested labels
			obj: &kapi.ReplicationController{
				ObjectMeta: kapi.ObjectMeta{
					Labels: map[string]string{"foo": "same value"},
				},
				Spec: kapi.ReplicationControllerSpec{
					Template: &kapi.PodTemplateSpec{
						ObjectMeta: kapi.ObjectMeta{
							Labels: map[string]string{"foo": "same value"},
						},
					},
				},
			},
			addLabels:      map[string]string{"foo": "same value"},
			err:            false,
			expectedLabels: map[string]string{"foo": "same value"},
		},
		{ // [9] Test conflicting keys with a different value in ReplicationController nested labels
			obj: &kapi.ReplicationController{
				ObjectMeta: kapi.ObjectMeta{
					Labels: map[string]string{"foo": "bar"},
				},
				Spec: kapi.ReplicationControllerSpec{
					Template: &kapi.PodTemplateSpec{
						ObjectMeta: kapi.ObjectMeta{
							Labels: map[string]string{"foo": "bar"},
						},
					},
					Selector: map[string]string{"foo": "bar"},
				},
			},
			addLabels:      map[string]string{"baz": ""},
			err:            false,
			expectedLabels: map[string]string{"foo": "bar", "baz": ""},
		},
		{ // [10] Test conflicting keys with a different value in ReplicationController nested labels
			obj: &kapi.ReplicationController{
				ObjectMeta: kapi.ObjectMeta{
					Labels: map[string]string{"foo": "first value"},
				},
				Spec: kapi.ReplicationControllerSpec{
					Template: &kapi.PodTemplateSpec{
						ObjectMeta: kapi.ObjectMeta{
							Labels: map[string]string{"foo": "first value"},
						},
					},
					Selector: map[string]string{"foo": "first value"},
				},
			},
			addLabels:      map[string]string{"foo": "second value"},
			err:            true,
			expectedLabels: map[string]string{"foo": "first value"},
		},
		{ // [11] Test adding labels to a Deployment object
			obj: &deployapi.Deployment{
				ObjectMeta: kapi.ObjectMeta{
					Labels: map[string]string{"foo": "first value"},
				},
				ControllerTemplate: kapi.ReplicationControllerSpec{
					Template: &kapi.PodTemplateSpec{
						ObjectMeta: kapi.ObjectMeta{
							Labels: map[string]string{"foo": "first value"},
						},
					},
				},
			},
			addLabels:      map[string]string{"bar": "second value"},
			err:            false,
			expectedLabels: map[string]string{"foo": "first value", "bar": "second value"},
		},
		{ // [12] Test adding labels to a DeploymentConfig object
			obj: &deployapi.DeploymentConfig{
				ObjectMeta: kapi.ObjectMeta{
					Labels: map[string]string{"foo": "first value"},
				},
				Template: deployapi.DeploymentTemplate{
					ControllerTemplate: kapi.ReplicationControllerSpec{
						Template: &kapi.PodTemplateSpec{
							ObjectMeta: kapi.ObjectMeta{
								Labels: map[string]string{"foo": "first value"},
							},
						},
					},
				},
			},
			addLabels:      map[string]string{"bar": "second value"},
			err:            false,
			expectedLabels: map[string]string{"foo": "first value", "bar": "second value"},
		},
		{ // [13] Test unknown Generic Object with Labels field
			obj: &FakeLabelsResource{
				ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{"baz": ""}},
			},
			addLabels:      map[string]string{"foo": "bar"},
			err:            false,
			expectedLabels: map[string]string{"foo": "bar", "baz": ""},
		},
	}

	for i, test := range testCases {
		err := AddObjectLabels(test.obj, test.addLabels)
		if err != nil && !test.err {
			t.Errorf("Unexpected error while setting labels on testCase[%v]: %v.", i, err)
		} else if err == nil && test.err {
			t.Errorf("Unexpected non-error while setting labels on testCase[%v].", i)
		}

		accessor, err := kmeta.Accessor(test.obj)
		if err != nil {
			t.Error(err)
		}
		metaLabels := accessor.Labels()
		if e, a := test.expectedLabels, metaLabels; !reflect.DeepEqual(e, a) {
			t.Errorf("Unexpected labels on testCase[%v]. Expected: %#v, got: %#v.", i, e, a)
		}

		// Handle nested Labels
		switch objType := test.obj.(type) {
		case *kapi.ReplicationController:
			if e, a := test.expectedLabels, objType.Spec.Template.Labels; !reflect.DeepEqual(e, a) {
				t.Errorf("Unexpected labels on testCase[%v]. Expected: %#v, got: %#v.", i, e, a)
			}
			if e, a := test.expectedLabels, objType.Spec.Selector; !reflect.DeepEqual(e, a) {
				t.Errorf("Unexpected labels on testCase[%v]. Expected: %#v, got: %#v.", i, e, a)
			}
		case *deployapi.Deployment:
			if e, a := test.expectedLabels, objType.ControllerTemplate.Template.Labels; !reflect.DeepEqual(e, a) {
				t.Errorf("Unexpected labels on testCase[%v]. Expected: %#v, got: %#v.", i, e, a)
			}
		case *deployapi.DeploymentConfig:
			if e, a := test.expectedLabels, objType.Template.ControllerTemplate.Template.Labels; !reflect.DeepEqual(e, a) {
				t.Errorf("Unexpected labels on testCase[%v]. Expected: %#v, got: %#v.", i, e, a)
			}
		}
	}
}
