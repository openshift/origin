package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/route"
	"github.com/openshift/origin/pkg/route/api"
	rest "github.com/openshift/origin/pkg/route/registry/route"
)

type REST struct {
	*etcdgeneric.Etcd
}

// NewREST returns a RESTStorage object that will work against routes.
func NewREST(s storage.Interface, allocator route.RouteAllocator) *REST {
	strategy := rest.NewStrategy(allocator)
	prefix := "/routes"
	store := &etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.Route{} },
		NewListFunc: func() runtime.Object { return &api.RouteList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdgeneric.NamespaceKeyRootFunc(ctx, prefix)
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return etcdgeneric.NamespaceKeyFunc(ctx, prefix, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.Route).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return rest.MatchRoute(label, field)
		},
		EndpointName: "routes",

		CreateStrategy: strategy,
		UpdateStrategy: strategy,

		Storage: s,
	}
	return &REST{store}
}
