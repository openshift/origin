package etcd

import (
	"errors"
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

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
)

// REST implements a RESTStorage for images against etcd.
type REST struct {
	store  *etcdgeneric.Etcd
	status *etcdgeneric.Etcd
}

// NewREST returns a new REST.
func NewREST(s storage.Interface) (*REST, *StatusREST, *FinalizeREST) {
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
			return prefix + "/" + name, nil
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.Image).Name, nil
		},
		EndpointName: "image",

		CreateStrategy: image.Strategy,
		UpdateStrategy: image.Strategy,

		ReturnDeletedObject: true,

		Storage: s,
	}

	statusStore := *store
	statusStore.UpdateStrategy = image.StatusStrategy

	finalizeStore := *store
	finalizeStore.UpdateStrategy = image.FinalizeStrategy

	return &REST{store: store, status: &statusStore}, &StatusREST{store: &statusStore}, &FinalizeREST{store: &finalizeStore}
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
	imgObj, err := r.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	img := imgObj.(*api.Image)
	// TODO: don't treat external images differently here -- take care of them with image controller
	isExternal := false
	if img.Annotations == nil {
		isExternal = true
	} else if value, ok := img.Annotations[api.ManagedByOpenShiftAnnotation]; !ok || value != "true" {
		isExternal = true
	}
	if isExternal {
		return r.store.Delete(ctx, name, nil)
	}

	// upon first request to delete an internally managed image, we switch the phase to start image termination
	if img.DeletionTimestamp.IsZero() {
		now := util.Now()
		img.DeletionTimestamp = &now
		img.Status.Phase = api.ImagePurging
		result, _, err := r.status.Update(ctx, img)
		return result, err
	}

	// prior to final deletion, we must ensure that finalizers is empty
	if len(img.Finalizers) != 0 {
		err = kapierrors.NewConflict("Image", img.Name, fmt.Errorf("The system is deleting image layers. Upon completion, this image will automatically be purged."))
		return nil, err
	}
	return r.store.Delete(ctx, name, nil)
}

// StatusREST implements the REST endpoint for changing the status of an image.
type StatusREST struct {
	store *etcdgeneric.Etcd
}

func (r *StatusREST) New() runtime.Object {
	return r.store.New()
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}

// FinalizeREST implements the REST endpoint for finalizing an image.
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
