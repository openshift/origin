package rolebinding

import (
	"errors"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/test"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

func testNewBaseBindings() []authorizationapi.PolicyBinding {
	return []authorizationapi.PolicyBinding{
		{
			ObjectMeta: kapi.ObjectMeta{Name: bootstrappolicy.DefaultMasterAuthorizationNamespace, Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
			PolicyRef:  kapi.ObjectReference{Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
			RoleBindings: map[string]authorizationapi.RoleBinding{
				"cluster-admins": {
					ObjectMeta: kapi.ObjectMeta{Name: "cluster-admins"},
					RoleRef:    kapi.ObjectReference{Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace, Name: "cluster-admin"},
					Users:      util.NewStringSet("system:admin"),
				},
			},
		},
		{
			ObjectMeta:   kapi.ObjectMeta{Name: bootstrappolicy.DefaultMasterAuthorizationNamespace, Namespace: "unittest"},
			PolicyRef:    kapi.ObjectReference{Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
			RoleBindings: map[string]authorizationapi.RoleBinding{},
		},
	}
}
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
	}
}

func makeTestStorage() *REST {
	bindingRegistry := test.NewPolicyBindingRegistry(testNewBaseBindings(), nil)
	policyRegistry := test.NewPolicyRegistry(testNewBasePolicies(), nil)

	return &REST{NewVirtualRegistry(bindingRegistry, policyRegistry, bootstrappolicy.DefaultMasterAuthorizationNamespace)}
}

func TestCreateValidationError(t *testing.T) {
	storage := makeTestStorage()
	roleBinding := &authorizationapi.RoleBinding{}

	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	_, err := storage.Create(ctx, roleBinding)
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestCreateValidAutoCreateMasterPolicyBindings(t *testing.T) {
	storage := makeTestStorage()
	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin", Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
	}

	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	obj, err := storage.Create(ctx, roleBinding)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	switch r := obj.(type) {
	case *kapi.Status:
		t.Errorf("Got back unexpected status: %#v", r)
	case *authorizationapi.RoleBinding:
		// expected case
	default:
		t.Errorf("Got unexpected type: %#v", r)
	}
}

func TestCreateValid(t *testing.T) {
	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})

	storage := makeTestStorage()
	storage.Create(ctx, &authorizationapi.PolicyBinding{
		ObjectMeta: kapi.ObjectMeta{Name: bootstrappolicy.DefaultMasterAuthorizationNamespace, Namespace: "unittest"},
	})

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin", Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
	}

	obj, err := storage.Create(ctx, roleBinding)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	switch obj.(type) {
	case *kapi.Status:
		t.Errorf("Got back unexpected status: %#v", obj)
	case *authorizationapi.RoleBinding:
		// expected case
	default:
		t.Errorf("Got unexpected type: %#v", obj)
	}
}

func TestUpdate(t *testing.T) {
	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})

	storage := makeTestStorage()
	storage.Create(ctx, &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin", Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
	})

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin", Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
	}

	obj, created, err := storage.Update(ctx, roleBinding)
	if err != nil || created {
		t.Errorf("Unexpected error %v", err)
	}

	switch obj.(type) {
	case *kapi.Status:
		t.Errorf("Unexpected operation error: %v", obj)

	case *authorizationapi.RoleBinding:
		if !reflect.DeepEqual(roleBinding, obj) {
			t.Errorf("Updated roleBinding does not match input roleBinding."+
				" Expected: %v, Got: %v", roleBinding, obj)
		}
	default:
		t.Errorf("Unexpected result type: %v", obj)
	}
}

func TestUpdateError(t *testing.T) {
	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})

	storage := makeTestStorage()
	storage.Create(ctx, &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-different"},
		RoleRef:    kapi.ObjectReference{Name: "admin", Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
	})

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin", Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
	}

	_, _, err := storage.Update(ctx, roleBinding)
	if err == nil {
		t.Errorf("Missing expected error")
		return
	}
	if !kapierrors.IsNotFound(err) {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestUpdateCannotChangeRoleRefError(t *testing.T) {
	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})

	storage := makeTestStorage()
	storage.Create(ctx, &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-different"},
		RoleRef:    kapi.ObjectReference{Name: "admin", Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
	})

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-different"},
		RoleRef:    kapi.ObjectReference{Name: "cluster-admin", Namespace: bootstrappolicy.DefaultMasterAuthorizationNamespace},
	}

	_, _, err := storage.Update(ctx, roleBinding)
	if err == nil {
		t.Errorf("Missing expected error")
		return
	}
	expectedErr := "roleBinding.RoleRef may not be modified"
	if err.Error() != expectedErr {
		t.Errorf("Expected %v, got %v", expectedErr, err)
	}
}

func TestDeleteError(t *testing.T) {
	registry := &test.PolicyBindingRegistry{}

	registry.Err = errors.New("Sample Error")
	policyRegistry := &test.PolicyRegistry{}
	storage := &REST{NewVirtualRegistry(registry, policyRegistry, bootstrappolicy.DefaultMasterAuthorizationNamespace)}

	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	_, err := storage.Delete(ctx, "foo")
	if err != registry.Err {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteValid(t *testing.T) {
	storage := makeTestStorage()

	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), bootstrappolicy.DefaultMasterAuthorizationNamespace), &user.DefaultInfo{Name: "system:admin"})
	obj, err := storage.Delete(ctx, "cluster-admins")
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
