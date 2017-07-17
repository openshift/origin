package test

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/kubectl"
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
	return "", fmt.Errorf("unexpected call to ScaleSimple")
}

type FakeLaggedScaler struct {
	Events     []ScaleEvent
	RetryCount int
}

func (t *FakeLaggedScaler) Scale(namespace, name string, newSize uint, preconditions *kubectl.ScalePrecondition, retry, wait *kubectl.RetryParams) error {
	if t.RetryCount != 2 {
		t.RetryCount += 1
		// This is faking a real error from the
		// "k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle" package.
		return errors.NewForbidden(schema.GroupResource{Resource: "ReplicationController"}, name, fmt.Errorf("%s: not yet ready to handle request", name))
	}
	t.Events = append(t.Events, ScaleEvent{name, newSize})
	return nil
}

func (t *FakeLaggedScaler) ScaleSimple(namespace, name string, preconditions *kubectl.ScalePrecondition, newSize uint) (string, error) {
	return "", nil
}
