package role

import (
	"errors"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/test"
)

func TestCreateValidationError(t *testing.T) {
	registry := &test.PolicyRegistry{}
	storage := REST{
		registry: registry,
	}
	role := &authorizationapi.Role{}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, err := storage.Create(ctx, role)
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestCreateStorageError(t *testing.T) {
	registry := &test.PolicyRegistry{}
	registry.Err = errors.New("Sample Error")

	storage := REST{
		registry: registry,
	}
	role := &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, err := storage.Create(ctx, role)
	if err == nil {
		t.Errorf("Missing expected error")
		return
	}
	if err != registry.Err {
		t.Errorf("Expected %v, got %v", registry.Err, err)
	}
}

func TestCreateValid(t *testing.T) {
	registry := &test.PolicyRegistry{}
	storage := REST{
		registry: registry,
	}
	registry.Policies = append(make([]authorizationapi.Policy, 0),
		authorizationapi.Policy{
			ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName, Namespace: "unittest"},
		})

	role := &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	obj, err := storage.Create(ctx, role)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	switch r := obj.(type) {
	case *kapi.Status:
		t.Errorf("Got back unexpected status: %#v", r)
	case *authorizationapi.Role:
		// expected case
	default:
		t.Errorf("Got unexpected type: %#v", r)
	}
}

func TestUpdate(t *testing.T) {
	registry := &test.PolicyRegistry{}
	storage := REST{
		registry: registry,
	}
	registry.Policies = append(make([]authorizationapi.Policy, 0),
		authorizationapi.Policy{
			ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName, Namespace: "unittest"},
			Roles: map[string]authorizationapi.Role{
				"my-role": {ObjectMeta: kapi.ObjectMeta{Name: "my-role"}},
			},
		})

	role := &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	obj, created, err := storage.Update(ctx, role)
	if err != nil || created {
		t.Errorf("Unexpected error %v", err)
	}

	switch obj.(type) {
	case *kapi.Status:
		t.Errorf("Unexpected operation error: %v", obj)

	case *authorizationapi.Role:
		if !reflect.DeepEqual(role, obj) {
			t.Errorf("Updated role does not match input role."+
				" Expected: %v, Got: %v", role, obj)
		}
	default:
		t.Errorf("Unexpected result type: %v", obj)
	}
}

func TestUpdateError(t *testing.T) {
	registry := &test.PolicyRegistry{}
	storage := REST{
		registry: registry,
	}
	registry.Policies = append(make([]authorizationapi.Policy, 0),
		authorizationapi.Policy{
			ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName, Namespace: "unittest"},
		})

	role := &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, _, err := storage.Update(ctx, role)
	if err == nil {
		t.Errorf("Missing expected error")
		return
	}
	expectedErr := "role my-role does not exist"
	if err.Error() != expectedErr {
		t.Errorf("Expected %v, got %v", expectedErr, err)
	}
}

func TestDeleteError(t *testing.T) {
	registry := &test.PolicyRegistry{}

	registry.Err = errors.New("Sample Error")
	storage := REST{
		registry: registry,
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, err := storage.Delete(ctx, "foo")
	if err != registry.Err {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteValid(t *testing.T) {
	registry := &test.PolicyRegistry{}
	storage := REST{
		registry: registry,
	}
	registry.Policies = append(make([]authorizationapi.Policy, 0),
		authorizationapi.Policy{
			ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName, Namespace: "unittest"},
			Roles: map[string]authorizationapi.Role{
				"foo": {ObjectMeta: kapi.ObjectMeta{Name: "foo"}},
			},
		})

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	obj, err := storage.Delete(ctx, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	switch r := obj.(type) {
	case *kapi.Status:
		if r.Status != "Success" {
			t.Fatalf("Got back non-success status: %#v", r)
		}
	default:
		t.Fatalf("Got back non-status result: %v", r)
	}
}
