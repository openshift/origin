package policybinding

import (
	"errors"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/test"
)

func TestCreateValidationError(t *testing.T) {
	registry := test.PolicyBindingRegistry{}
	storage := REST{
		registry: &registry,
	}
	policyBinding := &authorizationapi.PolicyBinding{
	// ObjectMeta: kapi.ObjectMeta{Name: "authTokenName"}, // Missing required field
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, err := storage.Create(ctx, policyBinding)
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestCreateStorageError(t *testing.T) {
	registry := test.PolicyBindingRegistry{
		Err: errors.New("Sample Error"),
	}
	storage := REST{
		registry: &registry,
	}
	policyBinding := &authorizationapi.PolicyBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "master"},
		PolicyRef:  kapi.ObjectReference{Namespace: "master"},
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, err := storage.Create(ctx, policyBinding)
	if err != registry.Err {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateValid(t *testing.T) {
	registry := test.PolicyBindingRegistry{}
	storage := REST{
		registry: &registry,
	}
	policyBinding := &authorizationapi.PolicyBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "master", Namespace: "unittest"},
		PolicyRef:  kapi.ObjectReference{Namespace: "master"},
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	obj, err := storage.Create(ctx, policyBinding)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	switch r := obj.(type) {
	case *kapi.Status:
		t.Errorf("Got back unexpected status: %#v", r)
	case *authorizationapi.PolicyBinding:
		// expected case
	default:
		t.Errorf("Got unexpected type: %#v", r)
	}
}

func TestGetError(t *testing.T) {
	registry := test.PolicyBindingRegistry{
		Err: errors.New("Sample Error"),
	}
	storage := REST{
		registry: &registry,
	}
	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, err := storage.Get(ctx, "name")
	if err == nil {
		t.Errorf("expected error")
		return
	}
	if err != registry.Err {
		t.Errorf("got unexpected error: %v", err)
		return
	}
}

func TestGetValid(t *testing.T) {
	registry := test.PolicyBindingRegistry{
		PolicyBindings: append(make([]authorizationapi.PolicyBinding, 0),
			authorizationapi.PolicyBinding{
				ObjectMeta: kapi.ObjectMeta{Name: "master", Namespace: "unittest"},
				PolicyRef:  kapi.ObjectReference{Namespace: "master"},
			}),
	}
	storage := REST{
		registry: &registry,
	}
	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	policyBinding, err := storage.Get(ctx, "master")
	if err != nil {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	if reflect.DeepEqual(policyBinding, registry.PolicyBindings[0]) {
		t.Errorf("got unexpected policyBinding: %v", policyBinding)
		return
	}
}

func TestListError(t *testing.T) {
	registry := test.PolicyBindingRegistry{
		Err: errors.New("Sample Error"),
	}
	storage := REST{
		registry: &registry,
	}
	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, err := storage.List(ctx, labels.Everything(), labels.Everything())
	if err == nil {
		t.Errorf("expected error")
		return
	}
	if err != registry.Err {
		t.Errorf("got unexpected error: %v", err)
		return
	}
}

func TestListEmpty(t *testing.T) {
	registry := test.PolicyBindingRegistry{
		PolicyBindings: make([]authorizationapi.PolicyBinding, 0),
	}
	storage := REST{
		registry: &registry,
	}
	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	policyBindings, err := storage.List(ctx, labels.Everything(), labels.Everything())
	if err != registry.Err {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	switch policyBindings := policyBindings.(type) {
	case *authorizationapi.PolicyBindingList:
		if len(policyBindings.Items) != 0 {
			t.Errorf("expected empty list, got %#v", policyBindings)
		}
	default:
		t.Errorf("expected policyBindingList, got: %v", policyBindings)
		return
	}
}

func TestList(t *testing.T) {
	registry := test.PolicyBindingRegistry{
		PolicyBindings: append(make([]authorizationapi.PolicyBinding, 0),
			authorizationapi.PolicyBinding{
				ObjectMeta: kapi.ObjectMeta{Name: "master", Namespace: "unittest"},
			}),
	}
	storage := REST{
		registry: &registry,
	}
	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	policyBindings, err := storage.List(ctx, labels.Everything(), labels.Everything())
	if err != registry.Err {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	switch policyBindings := policyBindings.(type) {
	case *authorizationapi.PolicyBindingList:
		if len(policyBindings.Items) != 1 {
			t.Errorf("expected list with 1 item, got %#v", policyBindings)
		}
	default:
		t.Errorf("expected policyBindingList, got: %v", policyBindings)
		return
	}
}

func TestDeleteError(t *testing.T) {
	registry := test.PolicyBindingRegistry{
		Err: errors.New("Sample Error"),
	}
	storage := REST{
		registry: &registry,
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, err := storage.Delete(ctx, "foo")
	if err != registry.Err {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteValid(t *testing.T) {
	registry := test.PolicyBindingRegistry{}
	storage := REST{
		registry: &registry,
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	obj, err := storage.Delete(ctx, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	switch r := obj.(type) {
	case *kapi.Status:
		if r.Status != "Success" {
			t.Errorf("Got back non-success status: %#v", r)
		}
	default:
		t.Errorf("Got back non-status result: %v", r)
	}

	if registry.DeletedPolicyBindingName != "foo" {
		t.Error("Unexpected policyBinding deleted: %s", registry.DeletedPolicyBindingName)
	}
}
