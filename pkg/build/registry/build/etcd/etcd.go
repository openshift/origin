package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/build"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// REST implements the REST endpoint for Builds
type REST struct {
	*registry.Store
}

// NewStorage returns a RESTStorage object that will work against Build objects.
func NewREST(optsGetter restoptions.Getter) (*REST, *DetailsREST, *StatusREST, error) {
	prefix := "/builds"

	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.Build{} },
		NewListFunc:       func() runtime.Object { return &api.BuildList{} },
		QualifiedResource: api.Resource("builds"),
		KeyRootFunc: func(ctx kapi.Context) string {
			return registry.NamespaceKeyRootFunc(ctx, prefix)
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return registry.NamespaceKeyFunc(ctx, prefix, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.Build).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return build.Matcher(label, field)
		},
		CreateStrategy:      build.Strategy,
		UpdateStrategy:      build.Strategy,
		DeleteStrategy:      build.Strategy,
		Decorator:           build.Decorator,
		ReturnDeletedObject: false,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, prefix); err != nil {
		return nil, nil, nil, err
	}

	detailsStore := *store
	detailsStore.UpdateStrategy = build.DetailsStrategy

	statusStore := *store
	statusStore.UpdateStrategy = build.StatusStrategy

	return &REST{store}, &DetailsREST{&detailsStore}, &StatusREST{&statusStore}, nil
}

// DetailsREST implements the REST endpoint for Build details
type DetailsREST struct {
	store *registry.Store
}

// New returns an empty object that can be used with Update after request data has been put into it.
func (r *DetailsREST) New() runtime.Object {
	return r.store.New()
}

// Update finds a resource in the storage and updates it.
func (r *DetailsREST) Update(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo)
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *DetailsREST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	return r.store.Get(ctx, name)
}

// StatusREST implements the REST endpoint for Build status
type StatusREST struct {
	store *registry.Store
}

// New returns an empty object that can be used with Update after request data has been put into it.
func (r *StatusREST) New() runtime.Object {
	return r.store.New()
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	return r.store.Get(ctx, name)
}

// Update finds a resource in the storage and updates it.
func (r *StatusREST) Update(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo)
}
