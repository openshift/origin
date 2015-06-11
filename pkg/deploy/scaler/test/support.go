package test

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
)

type FakeScaler struct {
	Events []ScaleEvent
}

type ScaleEvent struct {
	Name string
	Size uint
}

func (t *FakeScaler) Scale(namespace, name string, newSize uint, preconditions *kubectl.ScalePrecondition, retry, wait *kubectl.RetryParams) error {
	t.Events = append(t.Events, ScaleEvent{name, newSize})
	return nil
}

func (t *FakeScaler) ScaleSimple(namespace, name string, preconditions *kubectl.ScalePrecondition, newSize uint) (string, error) {
	return "error", fmt.Errorf("unexpected call to ScaleSimple")
}
