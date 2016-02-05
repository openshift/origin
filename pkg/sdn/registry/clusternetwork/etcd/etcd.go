package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/registry/clusternetwork"
)

// rest implements a RESTStorage for sdn against etcd
type REST struct {
	etcdgeneric.Etcd
}

const etcdPrefix = "/registry/sdnnetworks"

// NewREST returns a RESTStorage object that will work against subnets
func NewREST(s storage.Interface) *REST {
	store := &etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.ClusterNetwork{} },
		NewListFunc: func() runtime.Object { return &api.ClusterNetworkList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return etcdgeneric.NoNamespaceKeyFunc(ctx, etcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.ClusterNetwork).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return clusternetwork.Matcher(label, field)
		},
		QualifiedResource: api.Resource("clusternetwork"),

		Storage: s,
	}

	store.CreateStrategy = clusternetwork.Strategy
	store.UpdateStrategy = clusternetwork.Strategy

	return &REST{*store}
}
