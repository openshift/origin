package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/registry/clusternetwork"
)

// rest implements a RESTStorage for sdn against etcd
type REST struct {
	registry.Store
}

const etcdPrefix = "/registry/sdnnetworks"

// NewREST returns a RESTStorage object that will work against subnets
func NewREST(opts generic.RESTOptions) *REST {
	newListFunc := func() runtime.Object { return &api.ClusterNetworkList{} }
	storageInterface := opts.Decorator(opts.Storage, 100, &api.ClusterNetworkList{}, etcdPrefix, clusternetwork.Strategy, newListFunc)

	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.ClusterNetwork{} },
		NewListFunc: newListFunc,
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return registry.NoNamespaceKeyFunc(ctx, etcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.ClusterNetwork).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return clusternetwork.Matcher(label, field)
		},
		QualifiedResource: api.Resource("clusternetwork"),

		Storage: storageInterface,
	}

	store.CreateStrategy = clusternetwork.Strategy
	store.UpdateStrategy = clusternetwork.Strategy

	return &REST{*store}
}
