package role

import (
	"errors"
	"reflect"
	"testing"
	"time"

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
	channel, err := storage.Create(ctx, role)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case r := <-channel:
		switch r := r.Object.(type) {
		case *kapi.Status:
			t.Errorf("Got back unexpected status: %#v", r)
		case *authorizationapi.Role:
			// expected case
		default:
			t.Errorf("Got unexpected type: %#v", r)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
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
	channel, err := storage.Update(ctx, role)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	select {
	case result := <-channel:
		switch obj := result.Object.(type) {
		case *kapi.Status:
			t.Errorf("Unexpected operation error: %v", obj)

		case *authorizationapi.Role:
			if !reflect.DeepEqual(role, obj) {
				t.Errorf("Updated role does not match input role."+
					" Expected: %v, Got: %v", role, obj)
			}
		default:
			t.Errorf("Unexpected result type: %v", result)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
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
	_, err := storage.Update(ctx, role)
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
	channel, err := storage.Delete(ctx, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case r := <-channel:
		switch r := r.Object.(type) {
		case *kapi.Status:
			if r.Message == registry.Err.Error() {
				// expected case
			} else {
				t.Errorf("Got back unexpected error: %#v", r)
			}
		default:
			t.Errorf("Got back non-status result: %v", r)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
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
	channel, err := storage.Delete(ctx, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case r := <-channel:
		switch r := r.Object.(type) {
		case *kapi.Status:
			if r.Status != "Success" {
				t.Fatalf("Got back non-success status: %#v", r)
			}
		default:
			t.Fatalf("Got back non-status result: %v", r)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}
