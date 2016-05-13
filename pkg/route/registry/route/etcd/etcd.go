package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/route"
	"github.com/openshift/origin/pkg/route/api"
	rest "github.com/openshift/origin/pkg/route/registry/route"
)

type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against routes.
func NewREST(opts generic.RESTOptions, allocator route.RouteAllocator) (*REST, *StatusREST) {
	strategy := rest.NewStrategy(allocator)
	prefix := "/routes"

	newListFunc := func() runtime.Object { return &api.RouteList{} }
	storageInterface := opts.Decorator(opts.Storage, 100, &api.RouteList{}, prefix, strategy, newListFunc)

	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.Route{} },
		NewListFunc: func() runtime.Object { return &api.RouteList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return registry.NamespaceKeyRootFunc(ctx, prefix)
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return registry.NamespaceKeyFunc(ctx, prefix, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.Route).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return rest.Matcher(label, field)
		},
		QualifiedResource: api.Resource("routes"),

		CreateStrategy: strategy,
		UpdateStrategy: strategy,

		Storage: storageInterface,
	}

	statusStore := *store
	statusStore.UpdateStrategy = rest.StatusStrategy

	return &REST{store}, &StatusREST{&statusStore}
}

// StatusREST implements the REST endpoint for changing the status of a route.
type StatusREST struct {
	store *registry.Store
}

// New creates a new route resource
func (r *StatusREST) New() runtime.Object {
	return &api.Route{}
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}
