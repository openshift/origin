package policybased

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	rolebindingregistry "github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	"github.com/openshift/origin/pkg/authorization/registry/test"
)

func testNewClusterPolicies() []authorizationapi.ClusterPolicy {
	return []authorizationapi.ClusterPolicy{
		{
			ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName},
			Roles: map[string]*authorizationapi.ClusterRole{
				"cluster-admin": {
					ObjectMeta: kapi.ObjectMeta{Name: "cluster-admin"},
					Rules:      []authorizationapi.PolicyRule{{Verbs: sets.NewString("*"), Resources: sets.NewString("*")}},
				},
				"admin": {
					ObjectMeta: kapi.ObjectMeta{Name: "admin"},
					Rules:      []authorizationapi.PolicyRule{{Verbs: sets.NewString("*"), Resources: sets.NewString("*")}},
				},
			},
		},
	}
}

func testNewClusterBindings() []authorizationapi.ClusterPolicyBinding {
	return []authorizationapi.ClusterPolicyBinding{
		{
			ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.ClusterPolicyBindingName},
			RoleBindings: map[string]*authorizationapi.ClusterRoleBinding{
				"cluster-admins": {
					ObjectMeta: kapi.ObjectMeta{Name: "cluster-admins"},
					RoleRef:    kapi.ObjectReference{Name: "cluster-admin"},
					Subjects:   []kapi.ObjectReference{{Kind: authorizationapi.SystemUserKind, Name: "system:admin"}},
				},
			},
		},
	}
}
func testNewLocalBindings() []authorizationapi.PolicyBinding {
	return []authorizationapi.PolicyBinding{
		{
			ObjectMeta:   kapi.ObjectMeta{Name: authorizationapi.GetPolicyBindingName("unittest"), Namespace: "unittest"},
			RoleBindings: map[string]*authorizationapi.RoleBinding{},
		},
	}
}

func makeTestStorage() rolebindingregistry.Storage {
	clusterBindingRegistry := test.NewClusterPolicyBindingRegistry(testNewClusterBindings(), nil)
	bindingRegistry := test.NewPolicyBindingRegistry(testNewLocalBindings(), nil)
	clusterPolicyRegistry := test.NewClusterPolicyRegistry(testNewClusterPolicies(), nil)
	policyRegistry := test.NewPolicyRegistry([]authorizationapi.Policy{}, nil)

	return NewVirtualStorage(policyRegistry, bindingRegistry, clusterPolicyRegistry, clusterBindingRegistry)
}

func makeClusterTestStorage() rolebindingregistry.Storage {
	clusterBindingRegistry := test.NewClusterPolicyBindingRegistry(testNewClusterBindings(), nil)
	clusterPolicyRegistry := test.NewClusterPolicyRegistry(testNewClusterPolicies(), nil)

	return NewVirtualStorage(clusterpolicyregistry.NewSimulatedRegistry(clusterPolicyRegistry), clusterpolicybindingregistry.NewSimulatedRegistry(clusterBindingRegistry), clusterPolicyRegistry, clusterBindingRegistry)
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
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	}

	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	obj, err := storage.Create(ctx, roleBinding)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	switch r := obj.(type) {
	case *unversioned.Status:
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

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	}

	obj, err := storage.Create(ctx, roleBinding)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	switch obj.(type) {
	case *unversioned.Status:
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
	obj, err := storage.Create(ctx, &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	original := obj.(*authorizationapi.RoleBinding)

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding", ResourceVersion: original.ResourceVersion},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	}

	obj, created, err := storage.Update(ctx, roleBinding)
	if err != nil || created {
		t.Errorf("Unexpected error %v", err)
	}

	switch obj.(type) {
	case *unversioned.Status:
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
	obj, err := storage.Create(ctx, &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-different"},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	original := obj.(*authorizationapi.RoleBinding)

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-roleBinding", ResourceVersion: original.ResourceVersion},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	}

	_, _, err = storage.Update(ctx, roleBinding)
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
	obj, err := storage.Create(ctx, &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-different"},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	original := obj.(*authorizationapi.RoleBinding)

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: kapi.ObjectMeta{Name: "my-different", ResourceVersion: original.ResourceVersion},
		RoleRef:    kapi.ObjectReference{Name: "cluster-admin"},
	}

	_, _, err = storage.Update(ctx, roleBinding)
	if err == nil {
		t.Errorf("Missing expected error")
		return
	}
	expectedErr := "cannot change roleRef"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("Expected %v, got %v", expectedErr, err.Error())
	}
}

func TestDeleteError(t *testing.T) {
	bindingRegistry := &test.PolicyBindingRegistry{}
	bindingRegistry.Err = errors.New("sample Error")

	storage := NewVirtualStorage(&test.PolicyRegistry{}, bindingRegistry, &test.ClusterPolicyRegistry{}, &test.ClusterPolicyBindingRegistry{})

	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	_, err := storage.Delete(ctx, "foo", nil)
	if err != bindingRegistry.Err {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteValid(t *testing.T) {
	storage := makeClusterTestStorage()

	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), ""), &user.DefaultInfo{Name: "system:admin"})
	obj, err := storage.Delete(ctx, "cluster-admins", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	switch r := obj.(type) {
	case *unversioned.Status:
		if r.Status != "Success" {
			t.Fatalf("Got back non-success status: %#v", r)
		}
	default:
		t.Fatalf("Got back non-status result: %v", r)
	}
}
