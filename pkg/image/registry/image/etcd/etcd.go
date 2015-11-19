package etcd

import (
	"errors"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/watch"
)

// REST implements a RESTStorage for images against etcd.
type REST struct {
	store *etcdgeneric.Etcd
}

// NewREST returns a new REST.
func NewREST(s storage.Interface) *REST {
	prefix := "/images"
	store := &etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.Image{} },
		NewListFunc: func() runtime.Object { return &api.ImageList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			// images are not namespace scoped
			return prefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			// images are not namespace scoped
			return etcdgeneric.NoNamespaceKeyFunc(ctx, prefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.Image).Name, nil
		},
		EndpointName: "image",

		CreateStrategy: image.Strategy,
		UpdateStrategy: image.Strategy,

		ReturnDeletedObject: false,

		Storage: s,
	}
	return &REST{store: store}
}

// New returns a new object
func (r *REST) New() runtime.Object {
	return r.store.NewFunc()
}

// NewList returns a new list object
func (r *REST) NewList() runtime.Object {
	return r.store.NewListFunc()
}

// List obtains a list of images with labels that match selector.
func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	return r.store.ListPredicate(ctx, image.MatchImage(label, field))
}

// Watch begins watching for new, changed, or deleted images.
func (r *REST) Watch(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	if !field.Empty() {
		return nil, errors.New("field selectors are not supported on images")
	}
	return r.store.WatchPredicate(ctx, image.MatchImage(label, field), resourceVersion)
}

// Get gets a specific image specified by its ID.
func (r *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	return r.store.Get(ctx, name)
}

// Create creates an image based on a specification.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	return r.store.Create(ctx, obj)
}

// Update alters an existing image.
func (r *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}

// Delete deletes an existing image specified by its ID.
func (r *REST) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	return r.store.Delete(ctx, name, options)
}
