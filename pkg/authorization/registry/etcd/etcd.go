package etcd

import (
	"errors"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kmeta "github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	kubeetcd "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	etcdgeneric "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Etcd implements the Policy, AuthorizeToken, and Client registries backed by etcd.
type Etcd struct {
	policyRegistry        *etcdgeneric.Etcd
	policyBindingRegistry *etcdgeneric.Etcd
}

const (
	PolicyPath        = "/registry/authorization/policy"
	PolicyBindingPath = "/registry/authorization/policyBinding"
)

// New returns a new Etcd.
func New(helper tools.EtcdHelper) *Etcd {
	return &Etcd{
		policyRegistry: &etcdgeneric.Etcd{
			NewFunc:      func() runtime.Object { return &authorizationapi.Policy{} },
			NewListFunc:  func() runtime.Object { return &authorizationapi.PolicyList{} },
			EndpointName: "policy",
			KeyRootFunc: func(ctx kapi.Context) string {
				return etcdgeneric.NamespaceKeyRootFunc(ctx, PolicyPath)
			},
			KeyFunc: func(ctx kapi.Context, id string) (string, error) {
				return etcdgeneric.NamespaceKeyFunc(ctx, PolicyPath, id)
			},
			Helper: helper,
		},
		policyBindingRegistry: &etcdgeneric.Etcd{
			NewFunc:      func() runtime.Object { return &authorizationapi.PolicyBinding{} },
			NewListFunc:  func() runtime.Object { return &authorizationapi.PolicyBindingList{} },
			EndpointName: "policyBinding",
			KeyRootFunc: func(ctx kapi.Context) string {
				return etcdgeneric.NamespaceKeyRootFunc(ctx, PolicyBindingPath)
			},
			KeyFunc: func(ctx kapi.Context, id string) (string, error) {
				return etcdgeneric.NamespaceKeyFunc(ctx, PolicyBindingPath, id)
			},
			Helper: helper,
		},
	}
}

func getAttrs(obj runtime.Object) (klabels.Set, klabels.Set, error) {
	metaInterface, err := kmeta.Accessor(obj)
	if err != nil {
		return klabels.Set{}, klabels.Set{}, err
	}

	return metaInterface.Labels(), klabels.Set{}, nil
}

func (r *Etcd) GetPolicy(ctx kapi.Context, name string) (policy *authorizationapi.Policy, err error) {
	result, err := r.policyRegistry.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	ret, ok := result.(*authorizationapi.Policy)
	if !ok {
		return nil, errors.New("invalid object type")
	}

	return ret, nil
}

func (r *Etcd) ListPolicies(ctx kapi.Context, label, field klabels.Selector) (*authorizationapi.PolicyList, error) {
	result, err := r.policyRegistry.List(ctx, &generic.SelectionPredicate{label, field, getAttrs})
	if err != nil {
		return nil, err
	}
	ret, ok := result.(*authorizationapi.PolicyList)
	if !ok {
		return nil, errors.New("invalid object type")
	}

	return ret, nil
}

func (r *Etcd) CreatePolicy(ctx kapi.Context, policy *authorizationapi.Policy) error {
	return r.policyRegistry.Create(ctx, policy.Name, policy)
}

func (r *Etcd) UpdatePolicy(ctx kapi.Context, newPolicy *authorizationapi.Policy) error {
	return r.policyRegistry.Update(ctx, newPolicy.Name, newPolicy)
}

func (r *Etcd) DeletePolicy(ctx kapi.Context, name string) error {
	return r.policyRegistry.Delete(ctx, name)
}

func makePolicyBindingListKey(ctx kapi.Context) string {
	return kubeetcd.MakeEtcdListKey(ctx, PolicyBindingPath)
}

func makePolicyBindingKey(ctx kapi.Context, id string) (string, error) {
	return kubeetcd.MakeEtcdItemKey(ctx, PolicyBindingPath, id)
}

func (r *Etcd) GetPolicyBinding(ctx kapi.Context, name string) (policyBinding *authorizationapi.PolicyBinding, err error) {
	result, err := r.policyBindingRegistry.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	ret, ok := result.(*authorizationapi.PolicyBinding)
	if !ok {
		return nil, errors.New("invalid object type")
	}

	return ret, nil
}

func (r *Etcd) ListPolicyBindings(ctx kapi.Context, label, field klabels.Selector) (*authorizationapi.PolicyBindingList, error) {
	result, err := r.policyBindingRegistry.List(ctx, &generic.SelectionPredicate{label, field, getAttrs})
	if err != nil {
		return nil, err
	}
	ret, ok := result.(*authorizationapi.PolicyBindingList)
	if !ok {
		return nil, errors.New("invalid object type")
	}

	return ret, nil
}

func (r *Etcd) CreatePolicyBinding(ctx kapi.Context, binding *authorizationapi.PolicyBinding) error {
	return r.policyBindingRegistry.Create(ctx, binding.Name, binding)
}

func (r *Etcd) UpdatePolicyBinding(ctx kapi.Context, newPolicyBinding *authorizationapi.PolicyBinding) error {
	return r.policyBindingRegistry.Update(ctx, newPolicyBinding.Name, newPolicyBinding)
}

func (r *Etcd) DeletePolicyBinding(ctx kapi.Context, name string) error {
	return r.policyBindingRegistry.Delete(ctx, name)
}
