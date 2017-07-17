package policybased

import (
	"reflect"
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
	roleregistry "github.com/openshift/origin/pkg/authorization/registry/role"
	"github.com/openshift/origin/pkg/authorization/registry/test"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

func testNewLocalPolicies() []authorizationapi.Policy {
	return []authorizationapi.Policy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: authorizationapi.PolicyName, Namespace: "unittest"},
			Roles:      map[string]*authorizationapi.Role{},
		},
	}
}

func makeLocalTestStorage() roleregistry.Storage {
	policyRegistry := test.NewPolicyRegistry(testNewLocalPolicies(), nil)

	return NewVirtualStorage(policyRegistry, rulevalidation.NewDefaultRuleResolver(policyRegistry, &test.PolicyBindingRegistry{}, &test.ClusterPolicyRegistry{}, &test.ClusterPolicyBindingRegistry{}), nil)
}

func TestCreateValidationError(t *testing.T) {
	storage := makeLocalTestStorage()

	role := &authorizationapi.Role{}

	ctx := apirequest.WithNamespace(apirequest.NewContext(), "unittest")
	_, err := storage.Create(ctx, role, false)
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestCreateValid(t *testing.T) {
	storage := makeLocalTestStorage()

	role := &authorizationapi.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "my-role"},
	}

	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	obj, err := storage.Create(ctx, role, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	switch r := obj.(type) {
	case *metav1.Status:
		t.Errorf("Got back unexpected status: %#v", r)
	case *authorizationapi.Role:
		// expected case
	default:
		t.Errorf("Got unexpected type: %#v", r)
	}
}

func TestUpdate(t *testing.T) {
	storage := makeLocalTestStorage()
	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	realizedRoleObj, err := storage.Create(ctx, &authorizationapi.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "my-role"},
		Rules: []authorizationapi.PolicyRule{
			{Verbs: sets.NewString(authorizationapi.VerbAll)},
		},
	}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	realizedRole := realizedRoleObj.(*authorizationapi.Role)

	role := &authorizationapi.Role{
		ObjectMeta: realizedRole.ObjectMeta,
		Rules: []authorizationapi.PolicyRule{
			{Verbs: sets.NewString("list", "update")},
		},
	}

	obj, created, err := storage.Update(ctx, role.Name, rest.DefaultUpdatedObjectInfo(role, kapi.Scheme))
	if err != nil || created {
		t.Errorf("Unexpected error %v", err)
	}

	switch actual := obj.(type) {
	case *metav1.Status:
		t.Errorf("Unexpected operation error: %v", obj)

	case *authorizationapi.Role:
		if realizedRole.ResourceVersion == actual.ResourceVersion {
			t.Errorf("Expected change to role binding. Expected: %s, Got: %s", realizedRole.ResourceVersion, actual.ResourceVersion)
		}
		role.ResourceVersion = actual.ResourceVersion
		if !reflect.DeepEqual(role, obj) {
			t.Errorf("Updated role does not match input role. %s", diff.ObjectReflectDiff(role, obj))
		}
	default:
		t.Errorf("Unexpected result type: %v", obj)
	}
}

func TestUnconditionalUpdate(t *testing.T) {
	storage := makeLocalTestStorage()
	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	realizedRoleObj, err := storage.Create(ctx, &authorizationapi.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "my-role"},
		Rules: []authorizationapi.PolicyRule{
			{Verbs: sets.NewString(authorizationapi.VerbAll)},
		},
	}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	realizedRole := realizedRoleObj.(*authorizationapi.Role)

	role := &authorizationapi.Role{
		ObjectMeta: realizedRole.ObjectMeta,
		Rules: []authorizationapi.PolicyRule{
			{Verbs: sets.NewString("list", "update")},
		},
	}
	role.ResourceVersion = ""

	obj, created, err := storage.Update(ctx, role.Name, rest.DefaultUpdatedObjectInfo(role, kapi.Scheme))
	if err != nil || created {
		t.Errorf("Unexpected error %v", err)
	}

	switch actual := obj.(type) {
	case *metav1.Status:
		t.Errorf("Unexpected operation error: %v", obj)

	case *authorizationapi.Role:
		if realizedRole.ResourceVersion == actual.ResourceVersion {
			t.Errorf("Expected change to role binding. Expected: %s, Got: %s", realizedRole.ResourceVersion, actual.ResourceVersion)
		}
		role.ResourceVersion = actual.ResourceVersion
		if !reflect.DeepEqual(role, obj) {
			t.Errorf("Updated role does not match input role. %s", diff.ObjectReflectDiff(role, obj))
		}
	default:
		t.Errorf("Unexpected result type: %v", obj)
	}
}

func TestConflictingUpdate(t *testing.T) {
	storage := makeLocalTestStorage()
	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	realizedRoleObj, err := storage.Create(ctx, &authorizationapi.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "my-role"},
		Rules: []authorizationapi.PolicyRule{
			{Verbs: sets.NewString(authorizationapi.VerbAll)},
		},
	}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	realizedRole := realizedRoleObj.(*authorizationapi.Role)

	role := &authorizationapi.Role{
		ObjectMeta: realizedRole.ObjectMeta,
		Rules: []authorizationapi.PolicyRule{
			{Verbs: sets.NewString("list", "update")},
		},
	}
	role.ResourceVersion += "1"

	_, _, err = storage.Update(ctx, role.Name, rest.DefaultUpdatedObjectInfo(role, kapi.Scheme))
	if err == nil || !kapierrors.IsConflict(err) {
		t.Errorf("Expected conflict error, got: %#v", err)
	}
}

func TestUpdateNoOp(t *testing.T) {
	storage := makeLocalTestStorage()
	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	realizedRoleObj, err := storage.Create(ctx, &authorizationapi.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "my-role"},
		Rules: []authorizationapi.PolicyRule{
			{Verbs: sets.NewString(authorizationapi.VerbAll)},
		},
	}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	realizedRole := realizedRoleObj.(*authorizationapi.Role)

	role := &authorizationapi.Role{
		ObjectMeta: realizedRole.ObjectMeta,
		Rules: []authorizationapi.PolicyRule{
			{Verbs: sets.NewString(authorizationapi.VerbAll)},
		},
	}

	obj, created, err := storage.Update(ctx, role.Name, rest.DefaultUpdatedObjectInfo(role, kapi.Scheme))
	if err != nil || created {
		t.Errorf("Unexpected error %v", err)
	}

	switch o := obj.(type) {
	case *metav1.Status:
		t.Errorf("Unexpected operation error: %v", obj)

	case *authorizationapi.Role:
		if realizedRole.ResourceVersion != o.ResourceVersion {
			t.Errorf("Expected no change to role binding. Expected: %s, Got: %s", realizedRole.ResourceVersion, o.ResourceVersion)
		}
		if !reflect.DeepEqual(role, obj) {
			t.Errorf("Updated role does not match input role. %s", diff.ObjectReflectDiff(role, obj))
		}
	default:
		t.Errorf("Unexpected result type: %v", obj)
	}
}

func TestUpdateError(t *testing.T) {
	storage := makeLocalTestStorage()

	role := &authorizationapi.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "my-role"},
	}

	ctx := apirequest.WithNamespace(apirequest.NewContext(), "unittest")
	_, _, err := storage.Update(ctx, role.Name, rest.DefaultUpdatedObjectInfo(role, kapi.Scheme))
	if err == nil {
		t.Errorf("Missing expected error")
		return
	}
	if !kapierrors.IsNotFound(err) {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestDeleteError(t *testing.T) {
	storage := makeLocalTestStorage()

	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	_, _, err := storage.Delete(ctx, "foo", nil)

	if err == nil {
		t.Errorf("expected error")
	}
	if !kapierrors.IsNotFound(err) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteValid(t *testing.T) {
	storage := makeLocalTestStorage()
	ctx := apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	storage.Create(ctx, &authorizationapi.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "my-role"},
	}, false)

	obj, _, err := storage.Delete(ctx, "my-role", nil)
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
