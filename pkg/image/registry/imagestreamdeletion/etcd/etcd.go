package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/imagestreamdeletion"
)

// REST implements a RESTStorage for image stream deletions against etcd.
type REST struct {
	store *etcdgeneric.Etcd
}

// NewREST returns a new REST.
func NewREST(s storage.Interface) *REST {
	prefix := "/imagestreamdeletions"
	store := &etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.ImageStreamDeletion{} },
		NewListFunc: func() runtime.Object { return &api.ImageStreamDeletionList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			// image stream deletions are not namespace scoped
			return prefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			// image stream deletions are not namespace scoped
			return prefix + "/" + name, nil
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.ImageStreamDeletion).Name, nil
		},
		EndpointName: "imageStreamDeletion",

		ReturnDeletedObject: false,

		CreateStrategy: imagestreamdeletion.Strategy,

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

// List obtains a list of image stream deletions with labels that match selector.
func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	return r.store.ListPredicate(ctx, imagestreamdeletion.MatchImageStreamDeletion(label, field))
}

// Get gets a specific image stream deletion specified by its ID.
func (r *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	return r.store.Get(ctx, name)
}

// Create creates an image stream deletion based on a specification.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	return r.store.Create(ctx, obj)
}

// Delete deletes an existing image stream deletion specified by its ID.
func (r *REST) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	return r.store.Delete(ctx, name, options)
}
