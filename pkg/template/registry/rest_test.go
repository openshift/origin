package registry

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/api/v1beta1"
	config "github.com/openshift/origin/pkg/config/api"
	template "github.com/openshift/origin/pkg/template/api"
)

func TestNewRESTInvalidType(t *testing.T) {
	storage := NewREST()
	_, err := storage.Create(nil, &kapi.Pod{})
	if err == nil {
		t.Errorf("Expected type error.")
	}
}

func TestNewRESTDefaultsName(t *testing.T) {
	storage := NewREST()
	obj, err := storage.Create(nil, &template.Template{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, ok := obj.(*config.Config)
	if !ok {
		t.Fatalf("unexpected return object: %#v", obj)
	}
}

func TestNewRESTTemplateLabels(t *testing.T) {
	testLabels := map[string]string{
		"label1": "value1",
		"label2": "value2",
	}
	storage := NewREST()
	obj, err := storage.Create(nil, &template.Template{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test",
		},
		Objects: []runtime.Object{
			&kapi.Service{
				ObjectMeta: kapi.ObjectMeta{
					Name: "test-service",
				},
				Spec: kapi.ServiceSpec{
					Port:            80,
					Protocol:        kapi.ProtocolTCP,
					SessionAffinity: kapi.AffinityTypeNone,
				},
			},
		},
		ObjectLabels: testLabels,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	config, ok := obj.(*config.Config)
	if !ok {
		t.Fatalf("unexpected return object: %#v", obj)
	}
	svc, ok := config.Items[0].(*kapi.Service)
	if !ok {
		t.Fatalf("Unexpected object in config: %#v", svc)
	}
	for k, v := range testLabels {
		value, ok := svc.Labels[k]
		if !ok {
			t.Fatalf("Missing output label: %s", k)
		}
		if value != v {
			t.Fatalf("Unexpected label value: %s", value)
		}
	}
}

func TestRemoteRESTCreate(t *testing.T) {
	codec := v1beta1.Codec

	templateName := "testTemplate"
	tpl := &template.Template{
		ObjectMeta: kapi.ObjectMeta{
			Name: templateName,
		},
		Objects: []runtime.Object{
			&kapi.Service{
				ObjectMeta: kapi.ObjectMeta{
					Name: "test-service",
				},
				Spec: kapi.ServiceSpec{
					Port:            80,
					Protocol:        kapi.ProtocolTCP,
					SessionAffinity: kapi.AffinityTypeNone,
				},
			},
		},
	}

	r := RemoteREST{
		codec: codec,
		fetcher: &testFetcher{
			response: []byte(runtime.EncodeOrDie(codec, tpl)),
		},
	}

	remote := &template.RemoteTemplate{
		RemoteURL: "http://test.url/test",
	}

	obj, err := r.Create(nil, remote)
	if err != nil {
		t.Errorf("Unexpected error creating remoteTemplate: %v", err)
	}
	resultTpl, ok := obj.(*template.Template)
	if !ok {
		t.Errorf("Unexpected object %#v", obj)
	}
	if resultTpl.Name != templateName {
		t.Errorf("Unexpected template name: %s", resultTpl.Name)
	}
}

type testFetcher struct {
	response []byte
	isErr    bool
}

func (f *testFetcher) Fetch(URL string) ([]byte, error) {
	if f.isErr {
		return nil, fmt.Errorf("Error")
	}
	return f.response, nil
}
