package policy

import (
	"errors"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/test"
)

func TestGetError(t *testing.T) {
	registry := test.PolicyRegistry{
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
	testPolicy := authorizationapi.Policy{ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName, Namespace: "unittest"}}
	registry := test.NewPolicyRegistry([]authorizationapi.Policy{testPolicy}, nil)
	storage := REST{registry: registry}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	policy, err := storage.Get(ctx, authorizationapi.PolicyName)
	if err != nil {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	if reflect.DeepEqual(policy, testPolicy) {
		t.Errorf("got unexpected policy: %v", policy)
		return
	}
}

func TestListError(t *testing.T) {
	registry := test.PolicyRegistry{
		Err: errors.New("Sample Error"),
	}
	storage := REST{registry: &registry}

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
	registry := test.NewPolicyRegistry([]authorizationapi.Policy{}, nil)
	storage := REST{registry: registry}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	policies, err := storage.List(ctx, labels.Everything(), fields.Everything())
	if err != registry.Err {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	switch policies := policies.(type) {
	case *authorizationapi.PolicyList:
		if len(policies.Items) != 0 {
			t.Errorf("expected empty list, got %#v", policies)
		}
	default:
		t.Errorf("expected policyList, got: %v", policies)
		return
	}
}

func TestList(t *testing.T) {
	testPolicy := authorizationapi.Policy{ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName, Namespace: "unittest"}}
	registry := test.NewPolicyRegistry([]authorizationapi.Policy{testPolicy}, nil)
	storage := REST{registry: registry}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	policies, err := storage.List(ctx, labels.Everything(), fields.Everything())
	if err != registry.Err {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	switch policies := policies.(type) {
	case *authorizationapi.PolicyList:
		if len(policies.Items) != 1 {
			t.Errorf("expected list with 1 item, got %#v", policies)
		}
	default:
		t.Errorf("expected policyList, got: %v", policies)
		return
	}
}

func TestDeleteError(t *testing.T) {
	registry := test.PolicyRegistry{
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
	testPolicy := authorizationapi.Policy{ObjectMeta: kapi.ObjectMeta{Name: authorizationapi.PolicyName, Namespace: "unittest"}}
	registry := test.NewPolicyRegistry([]authorizationapi.Policy{testPolicy}, nil)
	storage := REST{registry: registry}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	obj, err := storage.Delete(ctx, authorizationapi.PolicyName)
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

	if policy, _ := registry.GetPolicy(ctx, authorizationapi.PolicyName); policy != nil {
		t.Error("Unexpected policy found: %v", policy)
	}
}
