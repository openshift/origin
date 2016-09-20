package etcd

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/registry/egressnetworkpolicy"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for egress network policy against etcd
type REST struct {
	registry.Store
}

// NewREST returns a RESTStorage object that will work against egress network policy
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.EgressNetworkPolicy{} },
		NewListFunc: func() runtime.Object { return &api.EgressNetworkPolicyList{} },
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.EgressNetworkPolicy).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
			return egressnetworkpolicy.Matcher(label, field)
		},
		QualifiedResource: api.Resource("egressnetworkpolicies"),

		CreateStrategy: egressnetworkpolicy.Strategy,
		UpdateStrategy: egressnetworkpolicy.Strategy,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, true, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return &REST{*store}, nil
}
