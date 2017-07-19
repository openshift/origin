package policybased

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	_ "github.com/openshift/origin/pkg/authorization/apis/authorization/install"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	rolebindingregistry "github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	"github.com/openshift/origin/pkg/authorization/registry/test"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

func testNewClusterPolicies() []authorizationapi.ClusterPolicy {
	return []authorizationapi.ClusterPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: authorizationapi.PolicyName},
			Roles: map[string]*authorizationapi.ClusterRole{
				"cluster-admin": {
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-admin"},
					Rules:      []authorizationapi.PolicyRule{{Verbs: sets.NewString("*"), Resources: sets.NewString("*")}},
				},
				"admin": {
					ObjectMeta: metav1.ObjectMeta{Name: "admin"},
					Rules:      []authorizationapi.PolicyRule{{Verbs: sets.NewString("*"), Resources: sets.NewString("*")}},
				},
			},
		},
	}
}

func testNewClusterBindings() []authorizationapi.ClusterPolicyBinding {
	return []authorizationapi.ClusterPolicyBinding{
		{
			ObjectMeta: metav1.ObjectMeta{Name: authorizationapi.ClusterPolicyBindingName},
			RoleBindings: map[string]*authorizationapi.ClusterRoleBinding{
				"cluster-admins": {
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-admins"},
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
			ObjectMeta:   metav1.ObjectMeta{Name: authorizationapi.GetPolicyBindingName("unittest"), Namespace: "unittest"},
			RoleBindings: map[string]*authorizationapi.RoleBinding{},
		},
	}
}

func makeTestStorage() rolebindingregistry.Storage {
	clusterBindingRegistry := test.NewClusterPolicyBindingRegistry(testNewClusterBindings(), nil)
	bindingRegistry := test.NewPolicyBindingRegistry(testNewLocalBindings(), nil)
	clusterPolicyRegistry := test.NewClusterPolicyRegistry(testNewClusterPolicies(), nil)
	policyRegistry := test.NewPolicyRegistry([]authorizationapi.Policy{}, nil)

	return NewVirtualStorage(bindingRegistry, rulevalidation.NewDefaultRuleResolver(policyRegistry, bindingRegistry, clusterPolicyRegistry, clusterBindingRegistry), nil)
}

func makeClusterTestStorage() rolebindingregistry.Storage {
	clusterBindingRegistry := test.NewClusterPolicyBindingRegistry(testNewClusterBindings(), nil)
	clusterPolicyRegistry := test.NewClusterPolicyRegistry(testNewClusterPolicies(), nil)
	bindingRegistry := clusterpolicybindingregistry.NewSimulatedRegistry(clusterBindingRegistry)

	return NewVirtualStorage(bindingRegistry, rulevalidation.NewDefaultRuleResolver(nil, nil, clusterPolicyRegistry, clusterBindingRegistry), nil)
}

func TestCreateValidationError(t *testing.T) {
	storage := makeTestStorage()
	roleBinding := &authorizationapi.RoleBinding{}

	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	_, err := storage.Create(ctx, roleBinding, false)
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestCreateValidAutoCreateMasterPolicyBindings(t *testing.T) {
	storage := makeTestStorage()
	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	}

	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	obj, err := storage.Create(ctx, roleBinding, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	switch r := obj.(type) {
	case *metav1.Status:
		t.Errorf("Got back unexpected status: %#v", r)
	case *authorizationapi.RoleBinding:
		// expected case
	default:
		t.Errorf("Got unexpected type: %#v", r)
	}
}

func TestCreateValid(t *testing.T) {
	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})

	storage := makeTestStorage()

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	}

	obj, err := storage.Create(ctx, roleBinding, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	switch obj.(type) {
	case *metav1.Status:
		t.Errorf("Got back unexpected status: %#v", obj)
	case *authorizationapi.RoleBinding:
		// expected case
	default:
		t.Errorf("Got unexpected type: %#v", obj)
	}
}

func TestUpdate(t *testing.T) {
	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})

	storage := makeTestStorage()
	obj, err := storage.Create(ctx, &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	}, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	original := obj.(*authorizationapi.RoleBinding)

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: original.ObjectMeta,
		RoleRef:    kapi.ObjectReference{Name: "admin"},
		Subjects:   []kapi.ObjectReference{{Name: "bob", Kind: "User"}},
	}

	obj, created, err := storage.Update(ctx, roleBinding.Name, rest.DefaultUpdatedObjectInfo(roleBinding, kapi.Scheme))
	if err != nil || created {
		t.Errorf("Unexpected error %v", err)
	}

	switch actual := obj.(type) {
	case *metav1.Status:
		t.Errorf("Unexpected operation error: %v", obj)

	case *authorizationapi.RoleBinding:
		if original.ResourceVersion == actual.ResourceVersion {
			t.Errorf("Expected change to role binding. Expected: %s, Got: %s", original.ResourceVersion, actual.ResourceVersion)
		}
		roleBinding.ResourceVersion = actual.ResourceVersion
		if !reflect.DeepEqual(roleBinding, obj) {
			t.Errorf("Updated roleBinding does not match input roleBinding. %s", diff.ObjectReflectDiff(roleBinding, obj))
		}
	default:
		t.Errorf("Unexpected result type: %v", obj)
	}
}

func TestUnconditionalUpdate(t *testing.T) {
	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})

	storage := makeTestStorage()
	obj, err := storage.Create(ctx, &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	}, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	original := obj.(*authorizationapi.RoleBinding)

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: original.ObjectMeta,
		RoleRef:    kapi.ObjectReference{Name: "admin"},
		Subjects:   []kapi.ObjectReference{{Name: "bob", Kind: "User"}},
	}
	roleBinding.ResourceVersion = ""

	obj, created, err := storage.Update(ctx, roleBinding.Name, rest.DefaultUpdatedObjectInfo(roleBinding, kapi.Scheme))
	if err != nil || created {
		t.Errorf("Unexpected error %v", err)
	}

	switch actual := obj.(type) {
	case *metav1.Status:
		t.Errorf("Unexpected operation error: %v", obj)

	case *authorizationapi.RoleBinding:
		if original.ResourceVersion == actual.ResourceVersion {
			t.Errorf("Expected change to role binding. Expected: %s, Got: %s", original.ResourceVersion, actual.ResourceVersion)
		}
		roleBinding.ResourceVersion = actual.ResourceVersion
		if !reflect.DeepEqual(roleBinding, obj) {
			t.Errorf("Updated roleBinding does not match input roleBinding. %s", diff.ObjectReflectDiff(roleBinding, obj))
		}
	default:
		t.Errorf("Unexpected result type: %v", obj)
	}
}

func TestConflictingUpdate(t *testing.T) {
	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})

	storage := makeTestStorage()
	obj, err := storage.Create(ctx, &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	}, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	original := obj.(*authorizationapi.RoleBinding)

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: original.ObjectMeta,
		RoleRef:    kapi.ObjectReference{Name: "admin"},
		Subjects:   []kapi.ObjectReference{{Name: "bob", Kind: "User"}},
	}
	roleBinding.ResourceVersion = roleBinding.ResourceVersion + "1"

	_, _, err = storage.Update(ctx, roleBinding.Name, rest.DefaultUpdatedObjectInfo(roleBinding, kapi.Scheme))
	if err == nil || !kapierrors.IsConflict(err) {
		t.Errorf("Expected conflict error, got: %#v", err)
	}
}

func TestUpdateNoOp(t *testing.T) {
	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})

	storage := makeTestStorage()
	obj, err := storage.Create(ctx, &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "my-roleBinding"},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	}, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	original := obj.(*authorizationapi.RoleBinding)

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: original.ObjectMeta,
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	}

	obj, created, err := storage.Update(ctx, roleBinding.Name, rest.DefaultUpdatedObjectInfo(roleBinding, kapi.Scheme))
	if err != nil || created {
		t.Errorf("Unexpected error %v", err)
	}

	switch o := obj.(type) {
	case *metav1.Status:
		t.Errorf("Unexpected operation error: %v", obj)

	case *authorizationapi.RoleBinding:
		if original.ResourceVersion != o.ResourceVersion {
			t.Errorf("Expected no change to role binding. Expected: %s, Got: %s", original.ResourceVersion, o.ResourceVersion)
		}
		if !reflect.DeepEqual(roleBinding, obj) {
			t.Errorf("Updated roleBinding does not match input roleBinding. %s", diff.ObjectReflectDiff(roleBinding, obj))
		}
	default:
		t.Errorf("Unexpected result type: %v", obj)
	}
}

func TestUpdateError(t *testing.T) {
	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})

	storage := makeTestStorage()
	obj, err := storage.Create(ctx, &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "my-different"},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	}, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	original := obj.(*authorizationapi.RoleBinding)

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "my-roleBinding", ResourceVersion: original.ResourceVersion},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	}

	_, _, err = storage.Update(ctx, roleBinding.Name, rest.DefaultUpdatedObjectInfo(roleBinding, kapi.Scheme))
	if err == nil {
		t.Errorf("Missing expected error")
		return
	}
	if !kapierrors.IsNotFound(err) {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestUpdateCannotChangeRoleRefError(t *testing.T) {
	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})

	storage := makeTestStorage()
	obj, err := storage.Create(ctx, &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "my-different"},
		RoleRef:    kapi.ObjectReference{Name: "admin"},
	}, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	original := obj.(*authorizationapi.RoleBinding)

	roleBinding := &authorizationapi.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "my-different", ResourceVersion: original.ResourceVersion},
		RoleRef:    kapi.ObjectReference{Name: "cluster-admin"},
	}

	_, _, err = storage.Update(ctx, roleBinding.Name, rest.DefaultUpdatedObjectInfo(roleBinding, kapi.Scheme))
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
	bindingRegistry.Err = errors.New("Sample Error")

	storage := NewVirtualStorage(bindingRegistry, rulevalidation.NewDefaultRuleResolver(&test.PolicyRegistry{}, bindingRegistry, &test.ClusterPolicyRegistry{}, &test.ClusterPolicyBindingRegistry{}), nil)
	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	_, _, err := storage.Delete(ctx, "foo", nil)
	if err != bindingRegistry.Err {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteValid(t *testing.T) {
	storage := makeClusterTestStorage()

	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), ""), &user.DefaultInfo{Name: "system:admin"})
	obj, _, err := storage.Delete(ctx, "cluster-admins", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	switch r := obj.(type) {
	case *metav1.Status:
		if r.Status != "Success" {
			t.Fatalf("Got back non-success status: %#v", r)
		}
	default:
		t.Fatalf("Got back non-status result: %v", r)
	}
}
