package template

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func TestNewRESTInvalidType(t *testing.T) {
	storage := NewREST()
	_, err := storage.Create(nil, &kapi.Pod{})
	if err == nil {
		t.Errorf("Expected type error.")
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
