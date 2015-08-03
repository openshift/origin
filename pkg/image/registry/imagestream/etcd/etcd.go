package etcd

import (
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/watch"
)

// REST implements a RESTStorage for image streams against etcd.
type REST struct {
	store                       *etcdgeneric.Etcd
	subjectAccessReviewRegistry subjectaccessreview.Registry
}

// NewREST returns a new REST.
func NewREST(s storage.Interface, defaultRegistry imagestream.DefaultRegistry, subjectAccessReviewRegistry subjectaccessreview.Registry) (*REST, *StatusREST) {
	prefix := "/imagestreams"
	store := etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.ImageStream{} },
		NewListFunc: func() runtime.Object { return &api.ImageStreamList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdgeneric.NamespaceKeyRootFunc(ctx, prefix)
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return etcdgeneric.NamespaceKeyFunc(ctx, prefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.ImageStream).Name, nil
		},
		EndpointName: "imageStream",

		ReturnDeletedObject: false,
		Storage:             s,
	}

	strategy := imagestream.NewStrategy(defaultRegistry, subjectAccessReviewRegistry)
	rest := &REST{subjectAccessReviewRegistry: subjectAccessReviewRegistry}
	strategy.ImageStreamGetter = rest

	statusStore := store
	statusStore.UpdateStrategy = imagestream.NewStatusStrategy(strategy)

	store.CreateStrategy = strategy
	store.UpdateStrategy = strategy
	store.Decorator = strategy.Decorate

	rest.store = &store

	return rest, &StatusREST{store: &statusStore}
}

// New returns a new object
func (r *REST) New() runtime.Object {
	return r.store.NewFunc()
}

// NewList returns a new list object
func (r *REST) NewList() runtime.Object {
	return r.store.NewListFunc()
}

// List obtains a list of image streams with labels that match selector.
func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	return r.store.ListPredicate(ctx, imagestream.MatchImageStream(label, field))
}

// Watch begins watching for new, changed, or deleted image streams.
func (r *REST) Watch(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return r.store.WatchPredicate(ctx, imagestream.MatchImageStream(label, field), resourceVersion)
}

// Get gets a specific image stream specified by its ID.
func (r *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	return r.store.Get(ctx, name)
}

// Create creates a image stream based on a specification.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	return r.store.Create(ctx, obj)
}

// Update changes a image stream specification.
func (r *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}

// UpdateAll updates both the image stream specification and status.
func (r *REST) UpdateAll(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	oldStrategy := r.store.UpdateStrategy
	r.store.UpdateStrategy = imagestream.NewAllStrategy(oldStrategy.(imagestream.Strategy))
	defer func() {
		r.store.UpdateStrategy = oldStrategy
	}()

	return r.store.Update(ctx, obj)
}

// Delete deletes an existing image stream specified by its ID.
func (r *REST) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	return r.store.Delete(ctx, name, options)
}

// StatusREST implements the REST endpoint for changing the status of an image stream.
type StatusREST struct {
	store *etcdgeneric.Etcd
}

func (r *StatusREST) New() runtime.Object {
	return &api.ImageStream{}
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}
