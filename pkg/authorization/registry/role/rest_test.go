package role

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/test"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

func testNewBasePolicies() []authorizationapi.Policy {
	return []authorizationapi.Policy{
		{
			ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName, Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
			Roles: map[string]authorizationapi.Role{
				"cluster-admin": {
					ObjectMeta: kapi.ObjectMeta{Name: "cluster-admin"},
					Rules:      []authorizationapi.PolicyRule{{Verbs: util.NewStringSet("*"), Resources: util.NewStringSet("*")}},
				},
				"admin": {
					ObjectMeta: kapi.ObjectMeta{Name: "admin"},
					Rules:      []authorizationapi.PolicyRule{{Verbs: util.NewStringSet("*"), Resources: util.NewStringSet("*")}},
				},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName, Namespace: "unittest"},
			Roles:      map[string]authorizationapi.Role{},
		},
	}
}

func makeTestStorage() *REST {
	policyRegistry := test.NewPolicyRegistry(testNewBasePolicies(), nil)

	return &REST{NewVirtualRegistry(policyRegistry)}
}

func TestCreateValidationError(t *testing.T) {
	storage := makeTestStorage()

	role := &authorizationapi.Role{}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, err := storage.Create(ctx, role)
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestCreateValid(t *testing.T) {
	storage := makeTestStorage()

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
	storage := makeTestStorage()
	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	storage.Create(ctx, &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
	})

	role := &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
	}

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
	storage := makeTestStorage()

	role := &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, _, err := storage.Update(ctx, role)
	if err == nil {
		t.Errorf("Missing expected error")
		return
	}
	if !kapierrors.IsNotFound(err) {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestDeleteError(t *testing.T) {
	storage := makeTestStorage()

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, err := storage.Delete(ctx, "foo")

	if err == nil {
		t.Errorf("expected error")
	}
	if !kapierrors.IsNotFound(err) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteValid(t *testing.T) {
	storage := makeTestStorage()
	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	storage.Create(ctx, &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
	})

	obj, err := storage.Delete(ctx, "my-role")
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
