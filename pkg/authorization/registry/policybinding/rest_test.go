package policybinding

import (
	"errors"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/test"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

func TestCreateValidationError(t *testing.T) {
	registry := test.NewPolicyBindingRegistry([]authorizationapi.PolicyBinding{}, nil)
	storage := REST{registry: registry}

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
	registry := test.NewPolicyBindingRegistry([]authorizationapi.PolicyBinding{}, nil)
	storage := REST{registry: registry}

	policyBinding := &authorizationapi.PolicyBinding{
		ObjectMeta: kapi.ObjectMeta{Name: bootstrappolicy.DefaultMasterAuthorizationNamespace},
		PolicyRef:  kapi.ObjectReference{Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, err := storage.Create(ctx, policyBinding)
	if err != registry.Err {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateValid(t *testing.T) {
	registry := test.NewPolicyBindingRegistry([]authorizationapi.PolicyBinding{}, nil)
	storage := REST{registry: registry}

	policyBinding := &authorizationapi.PolicyBinding{
		ObjectMeta: kapi.ObjectMeta{Name: bootstrappolicy.DefaultMasterAuthorizationNamespace, Namespace: "unittest"},
		PolicyRef:  kapi.ObjectReference{Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
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
	testBinding := authorizationapi.PolicyBinding{
		ObjectMeta: kapi.ObjectMeta{Name: bootstrappolicy.DefaultMasterAuthorizationNamespace, Namespace: "unittest"},
		PolicyRef:  kapi.ObjectReference{Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
	}
	registry := test.NewPolicyBindingRegistry([]authorizationapi.PolicyBinding{testBinding}, nil)
	storage := REST{registry: registry}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	policyBinding, err := storage.Get(ctx, bootstrappolicy.DefaultMasterAuthorizationNamespace)
	if err != nil {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	if reflect.DeepEqual(policyBinding, testBinding) {
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
	_, err := storage.List(ctx, labels.Everything(), fields.Everything())
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
	registry := test.NewPolicyBindingRegistry([]authorizationapi.PolicyBinding{}, nil)
	storage := REST{registry: registry}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	policyBindings, err := storage.List(ctx, labels.Everything(), fields.Everything())
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
	testBinding := authorizationapi.PolicyBinding{
		ObjectMeta: kapi.ObjectMeta{Name: bootstrappolicy.DefaultMasterAuthorizationNamespace, Namespace: "unittest"},
		PolicyRef:  kapi.ObjectReference{Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
	}
	registry := test.NewPolicyBindingRegistry([]authorizationapi.PolicyBinding{testBinding}, nil)
	storage := REST{registry: registry}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	policyBindings, err := storage.List(ctx, labels.Everything(), fields.Everything())
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
	testBinding := authorizationapi.PolicyBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "unittest"},
		PolicyRef:  kapi.ObjectReference{Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
	}
	registry := test.NewPolicyBindingRegistry([]authorizationapi.PolicyBinding{testBinding}, nil)
	storage := REST{registry: registry}

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

	if binding, _ := registry.GetPolicyBinding(ctx, "foo"); binding != nil {
		t.Error("Unexpected binding found: %v", binding)
	}
}
