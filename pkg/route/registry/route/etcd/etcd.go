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

type RouteStorage struct {
	Route  *REST
	Status *StatusREST
}

type REST struct {
	*etcdgeneric.Etcd
}

// NewREST returns a RESTStorage object that will work against routes.
func NewREST(s storage.Interface, allocator route.RouteAllocator) RouteStorage {
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
			return rest.Matcher(label, field)
		},
		EndpointName: "routes",

		CreateStrategy: strategy,
		UpdateStrategy: strategy,

		Storage: s,
	}
	return RouteStorage{
		Route:  &REST{store},
		Status: &StatusREST{store},
	}
}

// StatusREST implements the REST endpoint for changing the status of a route.
type StatusREST struct {
	store *etcdgeneric.Etcd
}

// New creates a new route resource
func (r *StatusREST) New() runtime.Object {
	return &api.Route{}
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}
