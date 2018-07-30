package template

import (
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	templatev1 "github.com/openshift/api/template/v1"
	"github.com/openshift/origin/pkg/template/apis/template"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	"k8s.io/apimachinery/pkg/runtime/serializer"
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

	testScheme := runtime.NewScheme()
	utilruntime.Must(v1.AddToScheme(testScheme))
	utilruntime.Must(templatev1.Install(testScheme))
	testCodec := serializer.NewCodecFactory(testScheme).LegacyCodec(templatev1.GroupVersion, v1.SchemeGroupVersion)
	// because of encoding changes, we to round-trip ourselves
	templateToCreate := &templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		ObjectLabels: testLabels,
	}
	templateToCreate.Objects = append(templateToCreate.Objects, runtime.RawExtension{
		Raw: []byte(runtime.EncodeOrDie(testCodec, &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-service",
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Port:     80,
						Protocol: v1.ProtocolTCP,
					},
				},
				SessionAffinity: v1.ServiceAffinityNone,
			},
		})),
	})

	originalBytes, err := runtime.Encode(testCodec, templateToCreate)
	if err != nil {
		t.Fatal(err)
	}
	objToCreate, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), originalBytes)
	if err != nil {
		t.Fatal(err)
	}
	internalTemplate := objToCreate.(*template.Template)
	obj, err := storage.Create(nil, internalTemplate, rest.ValidateAllObjectFunc, false)
	if err != nil {
		t.Fatal(err)
	}

	bytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Group: "template.openshift.io", Version: "v1"}), obj)
	if err != nil {
		t.Fatal(err)
	}
	obj, err = runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), bytes)
	if err != nil {
		t.Fatal(err)
	}

	config := obj.(*template.Template)
	if err := utilerrors.NewAggregate(runtime.DecodeList(config.Objects, legacyscheme.Codecs.UniversalDecoder())); err != nil {
		t.Fatal(err)
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

	testScheme := runtime.NewScheme()
	utilruntime.Must(v1.AddToScheme(testScheme))
	utilruntime.Must(templatev1.Install(testScheme))
	testCodec := serializer.NewCodecFactory(testScheme).LegacyCodec(templatev1.GroupVersion, v1.SchemeGroupVersion)
	// because of encoding changes, we to round-trip ourselves
	templateToCreate := &templatev1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		ObjectLabels: testLabels,
	}
	templateToCreate.Objects = append(templateToCreate.Objects, runtime.RawExtension{
		Raw: []byte(runtime.EncodeOrDie(testCodec, &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-service",
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Port:     80,
						Protocol: v1.ProtocolTCP,
					},
				},
				SessionAffinity: v1.ServiceAffinityNone,
			},
		})),
	})

	originalBytes, err := runtime.Encode(testCodec, templateToCreate)
	if err != nil {
		t.Fatal(err)
	}
	objToCreate, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), originalBytes)
	if err != nil {
		t.Fatal(err)
	}
	internalTemplate := objToCreate.(*template.Template)

	obj, err := storage.Create(nil, internalTemplate, rest.ValidateAllObjectFunc, false)
	if err != nil {
		t.Fatal(err)
	}

	bytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Group: "template.openshift.io", Version: "v1"}), obj)
	if err != nil {
		t.Fatal(err)
	}
	obj, err = runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), bytes)
	if err != nil {
		t.Fatal(err)
	}

	config := obj.(*template.Template)
	if err := utilerrors.NewAggregate(runtime.DecodeList(config.Objects, legacyscheme.Codecs.UniversalDecoder())); err != nil {
		t.Fatal(err)
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
