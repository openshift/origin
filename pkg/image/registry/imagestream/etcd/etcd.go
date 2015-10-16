package etcd

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
)

// REST implements a RESTStorage for image streams against etcd.
type REST struct {
	store                       *etcdgeneric.Etcd
	status                      *etcdgeneric.Etcd
	subjectAccessReviewRegistry subjectaccessreview.Registry
}

// NewREST returns a new REST.
func NewREST(s storage.Interface, defaultRegistry imagestream.DefaultRegistry, subjectAccessReviewRegistry subjectaccessreview.Registry) (*REST, *StatusREST, *FinalizeREST) {
	prefix := "/imagestreams"
	store := &etcdgeneric.Etcd{
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

		ReturnDeletedObject: true,
		Storage:             s,
	}

	strategy := imagestream.NewStrategy(defaultRegistry, subjectAccessReviewRegistry)
	rest := &REST{subjectAccessReviewRegistry: subjectAccessReviewRegistry}
	strategy.ImageStreamGetter = rest

	statusStore := *store
	statusStore.UpdateStrategy = imagestream.NewStatusStrategy(strategy)

	store.CreateStrategy = strategy
	store.UpdateStrategy = strategy
	store.Decorator = strategy.Decorate

	finalizeStore := *store
	finalizeStore.UpdateStrategy = imagestream.NewFinalizeStrategy(strategy)

	rest.store = store
	rest.status = &statusStore

	return rest, &StatusREST{store: &statusStore}, &FinalizeREST{store: &finalizeStore}
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

// Delete deletes an existing image stream specified by its ID.
func (r *REST) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	streamObj, err := r.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	stream := streamObj.(*api.ImageStream)

	// upon first request to delete an image, we switch the phase to start namespace termination
	if stream.DeletionTimestamp.IsZero() {
		now := util.Now()
		stream.DeletionTimestamp = &now
		stream.Status.Phase = api.ImageStreamTerminating
		result, _, err := r.status.Update(ctx, stream)
		return result, err
	}

	// prior to final deletion, we must ensure that finalizers is empty
	if len(stream.Spec.Finalizers) != 0 {
		err = kapierrors.NewConflict("ImageStream", stream.Name, fmt.Errorf("The system is scheduling dependent images for deletion. Upon completion, this image stream will automatically be purged by the system."))
		return nil, err
	}
	return r.store.Delete(ctx, name, nil)
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

// FinalizeREST implements the REST endpoint for finalizing an image stream.
type FinalizeREST struct {
	store *etcdgeneric.Etcd
}

func (r *FinalizeREST) New() runtime.Object {
	return r.store.New()
}

// Update alters the status finalizers subset of an object.
func (r *FinalizeREST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}
