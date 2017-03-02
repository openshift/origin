package etcd

import (
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/registry/egressnetworkpolicy"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for egress network policy against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against egress network policy
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.EgressNetworkPolicy{} },
		NewListFunc:       func() runtime.Object { return &api.EgressNetworkPolicyList{} },
		PredicateFunc:     egressnetworkpolicy.Matcher,
		QualifiedResource: api.Resource("egressnetworkpolicies"),

		CreateStrategy: egressnetworkpolicy.Strategy,
		UpdateStrategy: egressnetworkpolicy.Strategy,
	}

	// TODO this will be uncommented after 1.6 rebase:
	// options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: egressnetworkpolicy.GetAttrs}
	// if err := store.CompleteWithOptions(options); err != nil {
	if err := restoptions.ApplyOptions(optsGetter, store, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
