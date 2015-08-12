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
	"github.com/openshift/origin/pkg/sdn/registry/hostsubnet"
)

// rest implements a RESTStorage for sdn against etcd
type REST struct {
	etcdgeneric.Etcd
}

const etcdPrefix = "/registry/sdnsubnets"

// NewREST returns a RESTStorage object that will work against subnets
func NewREST(s storage.Interface) *REST {
	store := &etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.HostSubnet{} },
		NewListFunc: func() runtime.Object { return &api.HostSubnetList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return (etcdPrefix + "/" + name), nil
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.HostSubnet).Host, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return hostsubnet.MatchHostSubnet(label, field)
		},
		EndpointName: "hostsubnet",

		Storage: s,
	}

	store.CreateStrategy = hostsubnet.Strategy
	store.UpdateStrategy = hostsubnet.Strategy

	return &REST{*store}
}
