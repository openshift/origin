package template

import (
	"testing"
	"time"

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

	channel, err := storage.Delete(nil, "")
	if err != nil {
		t.Errorf("Unexpected error when deleting: %v", err)
	}
	select {
	case result := <-channel:
		if result.Object != nil {
			t.Errorf("Expected nil as the delete operation should not give any result")
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}
