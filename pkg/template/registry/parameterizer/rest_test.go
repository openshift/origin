package parameterizer

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/template/apis/template"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/api/install"
)

func TestTemplateInput(t *testing.T) {
	storage := NewREST()
	obj, err := storage.Create(nil, &template.ParameterizeTemplateRequest{
		Template: template.Template{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		},
	}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, ok := obj.(*template.Template)
	if !ok {
		t.Fatalf("unexpected return object: %#v", obj)
	}
}

func TestInvalidObjectInput(t *testing.T) {
	pod := &kapi.Pod{}
	storage := NewREST()
	_, err := storage.Create(nil, pod, false)
	if err == nil {
		t.Errorf("expected an error with invalid object")
	}
}
