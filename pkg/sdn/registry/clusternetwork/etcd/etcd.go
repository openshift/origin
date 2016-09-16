package etcd

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/registry/clusternetwork"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for sdn against etcd
type REST struct {
	registry.Store
}

// NewREST returns a RESTStorage object that will work against subnets
func NewREST(optsGetter restoptions.Getter) (*REST, error) {

	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.ClusterNetwork{} },
		NewListFunc: func() runtime.Object { return &api.ClusterNetworkList{} },
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.ClusterNetwork).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
			return clusternetwork.Matcher(label, field)
		},
		QualifiedResource: api.Resource("clusternetworks"),

		CreateStrategy: clusternetwork.Strategy,
		UpdateStrategy: clusternetwork.Strategy,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, false, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return &REST{*store}, nil
}
