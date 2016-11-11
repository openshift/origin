package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/build"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against Build objects.
func NewREST(optsGetter restoptions.Getter) (*REST, *DetailsREST, error) {

	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &api.Build{} },
		NewListFunc:       func() runtime.Object { return &api.BuildList{} },
		QualifiedResource: api.Resource("builds"),
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.Build).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
			return build.Matcher(label, field)
		},
		CreateStrategy:      build.Strategy,
		UpdateStrategy:      build.Strategy,
		DeleteStrategy:      build.Strategy,
		Decorator:           build.Decorator,
		ReturnDeletedObject: false,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, true, storage.NoTriggerPublisher); err != nil {
		return nil, nil, err
	}

	detailsStore := *store
	detailsStore.UpdateStrategy = build.DetailsStrategy

	return &REST{store}, &DetailsREST{&detailsStore}, nil
}

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
