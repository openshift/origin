package etcd

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	etcdgeneric "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
)

// REST implements a RESTStorage for image repositories against etcd.
type REST struct {
	store *etcdgeneric.Etcd
}

// NewREST returns a new REST.
func NewREST(h tools.EtcdHelper, defaultRegistry imagerepository.DefaultRegistry) (*REST, *StatusREST) {
	prefix := "/imageRepositories"
	store := etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.ImageRepository{} },
		NewListFunc: func() runtime.Object { return &api.ImageRepositoryList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdgeneric.NamespaceKeyRootFunc(ctx, prefix)
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return etcdgeneric.NamespaceKeyFunc(ctx, prefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.ImageRepository).Name, nil
		},
		EndpointName: "imageRepository",

		ReturnDeletedObject: false,
		Helper:              h,
	}

	strategy := imagerepository.NewStrategy(defaultRegistry)

	statusStore := store
	statusStore.UpdateStrategy = imagerepository.NewStatusStrategy(strategy)

	store.CreateStrategy = strategy
	store.UpdateStrategy = strategy
	store.Decorator = strategy.Decorate

	return &REST{store: &store}, &StatusREST{store: &statusStore}
}

// New returns a new object
func (r *REST) New() runtime.Object {
	return r.store.NewFunc()
}

// NewList returns a new list object
func (r *REST) NewList() runtime.Object {
	return r.store.NewListFunc()
}

// List obtains a list of image repositories with labels that match selector.
func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	return r.store.ListPredicate(ctx, imagerepository.MatchImageRepository(label, field))
}

// Watch begins watching for new, changed, or deleted image repositories.
func (r *REST) Watch(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return r.store.WatchPredicate(ctx, imagerepository.MatchImageRepository(label, field), resourceVersion)
}

// Get gets a specific image repository specified by its ID.
func (r *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	return r.store.Get(ctx, name)
}

// Create creates a image repository based on a specification.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	return r.store.Create(ctx, obj)
}

// Update changes a image repository specification.
func (r *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}

// Delete deletes an existing image repository specified by its ID.
func (r *REST) Delete(ctx kapi.Context, name string) (runtime.Object, error) {
	return r.store.Delete(ctx, name)
}

// StatusREST implements the REST endpoint for changing the status of an image repository.
type StatusREST struct {
	store *etcdgeneric.Etcd
}

func (r *StatusREST) New() runtime.Object {
	return &api.ImageRepository{}
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}
