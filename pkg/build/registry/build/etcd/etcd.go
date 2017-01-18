package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
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
		PredicateFunc:     build.Matcher,
		QualifiedResource: api.Resource("builds"),

		CreateStrategy: build.Strategy,
		UpdateStrategy: build.Strategy,
		DeleteStrategy: build.Strategy,
	}

	// TODO this will be uncommented after 1.6 rebase:
	// options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: build.GetAttrs}
	// if err := store.CompleteWithOptions(options); err != nil {
	if err := restoptions.ApplyOptions(optsGetter, store, storage.NoTriggerPublisher); err != nil {
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
