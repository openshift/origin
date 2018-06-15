package template

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	template "github.com/openshift/origin/pkg/template/apis/template"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
)

func TestNewRESTDefaultsName(t *testing.T) {
	storage := NewREST()
	obj, err := storage.Create(nil, &template.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}, rest.ValidateAllObjectFunc, false)
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
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Parameters: []template.Parameter{
			{
				Name:     "TEST_PARAM",
				Generate: "[a-z0-Z0-9]{8}",
			},
		},
		Objects: []runtime.Object{},
	}, rest.ValidateAllObjectFunc, false)
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
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		ObjectLabels: testLabels,
	}
	templateObjects := []runtime.Object{
		&kapi.Service{
			ObjectMeta: metav1.ObjectMeta{
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
	template.AddObjectsToTemplate(templateToCreate, templateObjects, legacyscheme.Registry.GroupOrDie(kapi.GroupName).GroupVersions[0])
	originalBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(legacyscheme.Registry.GroupOrDie(kapi.GroupName).GroupVersions[0]), templateToCreate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	objToCreate, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), originalBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	templateToCreate = objToCreate.(*template.Template)

	obj, err := storage.Create(nil, templateToCreate, rest.ValidateAllObjectFunc, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(legacyscheme.Registry.GroupOrDie(kapi.GroupName).GroupVersions[0]), obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	obj, err = runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), bytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config := obj.(*template.Template)
	if err := utilerrors.NewAggregate(runtime.DecodeList(config.Objects, legacyscheme.Codecs.UniversalDecoder())); err != nil {
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
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		ObjectLabels: testLabels,
	}
	templateObjects := []runtime.Object{
		&kapi.Service{
			ObjectMeta: metav1.ObjectMeta{
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
	template.AddObjectsToTemplate(templateToCreate, templateObjects, legacyscheme.Registry.GroupOrDie(kapi.GroupName).GroupVersions[0])
	originalBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(legacyscheme.Registry.GroupOrDie(kapi.GroupName).GroupVersions[0]), templateToCreate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	objToCreate, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), originalBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	templateToCreate = objToCreate.(*template.Template)

	obj, err := storage.Create(nil, templateToCreate, rest.ValidateAllObjectFunc, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(legacyscheme.Registry.GroupOrDie(kapi.GroupName).GroupVersions[0]), obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	obj, err = runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), bytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config := obj.(*template.Template)
	if err := utilerrors.NewAggregate(runtime.DecodeList(config.Objects, legacyscheme.Codecs.UniversalDecoder())); err != nil {
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
