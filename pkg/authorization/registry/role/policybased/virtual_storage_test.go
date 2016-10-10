package policybased

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util/diff"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	_ "github.com/openshift/origin/pkg/authorization/api/install"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	roleregistry "github.com/openshift/origin/pkg/authorization/registry/role"
	"github.com/openshift/origin/pkg/authorization/registry/test"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
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
func testNewLocalPolicies() []authorizationapi.Policy {
	return []authorizationapi.Policy{
		{
			ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName, Namespace: "unittest"},
			Roles:      map[string]*authorizationapi.Role{},
		},
	}
}

func makeLocalTestStorage() roleregistry.Storage {
	policyRegistry := test.NewPolicyRegistry(testNewLocalPolicies(), nil)

	return NewVirtualStorage(policyRegistry, rulevalidation.NewDefaultRuleResolver(policyRegistry, &test.PolicyBindingRegistry{}, &test.ClusterPolicyRegistry{}, &test.ClusterPolicyBindingRegistry{}), nil, authorizationapi.Resource("role"))
}

func makeClusterTestStorage() roleregistry.Storage {
	clusterPolicyRegistry := test.NewClusterPolicyRegistry(testNewClusterPolicies(), nil)
	policyRegistry := clusterpolicyregistry.NewSimulatedRegistry(clusterPolicyRegistry)

	return NewVirtualStorage(policyRegistry, rulevalidation.NewDefaultRuleResolver(nil, &test.PolicyBindingRegistry{}, clusterPolicyRegistry, &test.ClusterPolicyBindingRegistry{}), nil, authorizationapi.Resource("clusterrole"))
}

func TestCreateValidationError(t *testing.T) {
	storage := makeLocalTestStorage()

	role := &authorizationapi.Role{}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	_, err := storage.Create(ctx, role)
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestCreateValid(t *testing.T) {
	storage := makeLocalTestStorage()

	role := &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
	}

	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	obj, err := storage.Create(ctx, role)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	switch r := obj.(type) {
	case *unversioned.Status:
		t.Errorf("Got back unexpected status: %#v", r)
	case *authorizationapi.Role:
		// expected case
	default:
		t.Errorf("Got unexpected type: %#v", r)
	}
}

func TestUpdate(t *testing.T) {
	storage := makeLocalTestStorage()
	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	realizedRoleObj, err := storage.Create(ctx, &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
		Rules: []authorizationapi.PolicyRule{
			{Verbs: sets.NewString(authorizationapi.VerbAll)},
		},
	})
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
	case *unversioned.Status:
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
	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	realizedRoleObj, err := storage.Create(ctx, &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
		Rules: []authorizationapi.PolicyRule{
			{Verbs: sets.NewString(authorizationapi.VerbAll)},
		},
	})
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
	case *unversioned.Status:
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
	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	realizedRoleObj, err := storage.Create(ctx, &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
		Rules: []authorizationapi.PolicyRule{
			{Verbs: sets.NewString(authorizationapi.VerbAll)},
		},
	})
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
	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	realizedRoleObj, err := storage.Create(ctx, &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
		Rules: []authorizationapi.PolicyRule{
			{Verbs: sets.NewString(authorizationapi.VerbAll)},
		},
	})
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
	case *unversioned.Status:
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
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
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

	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	_, err := storage.Delete(ctx, "foo", nil)

	if err == nil {
		t.Errorf("expected error")
	}
	if !kapierrors.IsNotFound(err) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteValid(t *testing.T) {
	storage := makeLocalTestStorage()
	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "unittest"), &user.DefaultInfo{Name: "system:admin"})
	storage.Create(ctx, &authorizationapi.Role{
		ObjectMeta: kapi.ObjectMeta{Name: "my-role"},
	})

	obj, err := storage.Delete(ctx, "my-role", nil)
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
