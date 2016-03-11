package registry

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/runtime"
	utilerrors "k8s.io/kubernetes/pkg/util/errors"

	template "github.com/openshift/origin/pkg/template/api"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/api/install"
)

func TestNewRESTInvalidType(t *testing.T) {
	storage := NewREST()
	_, err := storage.Create(nil, &kapi.Pod{})
	if err == nil {
		t.Errorf("Expected type error.")
	}

	if _, err := registered.RESTMapper().KindFor(template.Resource("processedtemplates").WithVersion("")); err != nil {
		t.Errorf("no processed templates: %v", err)
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
	_, ok := obj.(*template.Template)
	if !ok {
		t.Fatalf("unexpected return object: %#v", obj)
	}
}

func TestNewRESTInvalidParameter(t *testing.T) {
	storage := NewREST()
	_, err := storage.Create(nil, &template.Template{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test",
		},
		Parameters: []template.Parameter{
			{
				Name:     "TEST_PARAM",
				Generate: "[a-z0-Z0-9]{8}",
			},
		},
		Objects: []runtime.Object{},
	})
	if err == nil {
		t.Fatalf("Expected 'invalid parameter error', got nothing")
	}
}

func TestNewRESTTemplateLabels(t *testing.T) {
	testLabels := map[string]string{
		"label1": "value1",
		"label2": "value2",
	}
	storage := NewREST()

	// because of encoding changes, we to round-trip ourselves
	templateToCreate := &template.Template{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test",
		},
		ObjectLabels: testLabels,
	}
	templateObjects := []runtime.Object{
		&kapi.Service{
			ObjectMeta: kapi.ObjectMeta{
				Name: "test-service",
			},
			Spec: kapi.ServiceSpec{
				Ports: []kapi.ServicePort{
					{
						Port:     80,
						Protocol: kapi.ProtocolTCP,
					},
				},
				SessionAffinity: kapi.ServiceAffinityNone,
			},
		},
	}
	template.AddObjectsToTemplate(templateToCreate, templateObjects, registered.GroupOrDie(kapi.GroupName).GroupVersions[0])
	originalBytes, err := runtime.Encode(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), templateToCreate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	objToCreate, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), originalBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	templateToCreate = objToCreate.(*template.Template)

	obj, err := storage.Create(nil, templateToCreate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bytes, err := runtime.Encode(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	obj, err = runtime.Decode(kapi.Codecs.UniversalDecoder(), bytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config := obj.(*template.Template)
	if err := utilerrors.NewAggregate(runtime.DecodeList(config.Objects, kapi.Codecs.UniversalDecoder())); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc, ok := config.Objects[0].(*kapi.Service)
	if !ok {
		t.Fatalf("Unexpected object in config: %#v", config.Objects[0])
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

func TestNewRESTTemplateLabelsList(t *testing.T) {
	testLabels := map[string]string{
		"label1": "value1",
		"label2": "value2",
	}
	storage := NewREST()
	// because of encoding changes, we to round-trip ourselves
	templateToCreate := &template.Template{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test",
		},
		ObjectLabels: testLabels,
	}
	templateObjects := []runtime.Object{
		&kapi.Service{
			ObjectMeta: kapi.ObjectMeta{
				Name: "test-service",
			},
			Spec: kapi.ServiceSpec{
				Ports: []kapi.ServicePort{
					{
						Port:     80,
						Protocol: kapi.ProtocolTCP,
					},
				},
				SessionAffinity: kapi.ServiceAffinityNone,
			},
		},
	}
	template.AddObjectsToTemplate(templateToCreate, templateObjects, registered.GroupOrDie(kapi.GroupName).GroupVersions[0])
	originalBytes, err := runtime.Encode(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), templateToCreate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	objToCreate, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), originalBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	templateToCreate = objToCreate.(*template.Template)

	obj, err := storage.Create(nil, templateToCreate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bytes, err := runtime.Encode(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	obj, err = runtime.Decode(kapi.Codecs.UniversalDecoder(), bytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config := obj.(*template.Template)
	if err := utilerrors.NewAggregate(runtime.DecodeList(config.Objects, kapi.Codecs.UniversalDecoder())); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc, ok := config.Objects[0].(*kapi.Service)
	if !ok {
		t.Fatalf("Unexpected object in config: %#v", config.Objects[0])
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
