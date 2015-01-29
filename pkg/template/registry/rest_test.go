package template

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

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
	ch, err := storage.Create(nil, &template.Template{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	obj := <-ch
	_, ok := obj.Object.(*config.Config)
	if !ok {
		t.Fatalf("unexpected return object: %#v", obj)
	}
}

func TestStorageNotImplementedFunctions(t *testing.T) {
	storage := NewREST()

	if _, err := storage.List(nil, nil, nil); err == nil {
		t.Errorf("Expected not implemented error.")
	}

	if _, err := storage.Get(nil, ""); err == nil {
		t.Errorf("Expected not implemented error.")
	}

	if _, err := storage.Update(nil, nil); err == nil {
		t.Errorf("Expected not implemented error.")
	}

	_, err := storage.Delete(nil, "")
	if err != nil {
		t.Errorf("Unexpected error when deleting: %v", err)
	}
}
