package etcd

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	etcdgeneric "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/storage"

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
			return (etcdPrefix + "/" + name), nil
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.ClusterNetwork).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return clusternetwork.MatchClusterNetwork(label, field)
		},
		EndpointName: "clusternetwork",

		Storage: s,
	}

	store.CreateStrategy = clusternetwork.Strategy
	store.UpdateStrategy = clusternetwork.Strategy

	return &REST{*store}
}
