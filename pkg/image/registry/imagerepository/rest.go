package imagerepository

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
)

type REST struct {
	imageStreamRegistry imagestream.Registry
}

func NewREST(isr imagestream.Registry) (*REST, *StatusREST) {
	return &REST{imageStreamRegistry: isr}, &StatusREST{imageStreamRegistry: isr}
}

// New returns a new object
func (r *REST) New() runtime.Object {
	return &api.ImageRepository{}
}

// NewList returns a new list object
func (r *REST) NewList() runtime.Object {
	return &api.ImageRepositoryList{}
}

// List obtains a list of image repositories with labels that match selector.
func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	imageStreamList, err := r.imageStreamRegistry.ListImageStreams(ctx, label)
	if err != nil {
		return nil, err
	}
	imageRepoList := api.ImageRepositoryList{}
	if err := kapi.Scheme.Convert(imageStreamList, &imageRepoList); err != nil {
		return nil, err
	}
	return &imageRepoList, nil
}

// Watch begins watching for new, changed, or deleted image repositories.
func (r *REST) Watch(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return r.imageStreamRegistry.WatchImageStreams(ctx, label, field, resourceVersion)
}

// Get gets a specific image repository specified by its ID.
func (r *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	stream, err := r.imageStreamRegistry.GetImageStream(ctx, name)
	if err != nil {
		return nil, err
	}

	repo := api.ImageRepository{}
	if err := kapi.Scheme.Convert(stream, &repo); err != nil {
		return nil, err
	}

	return &repo, nil
}

// Create creates a image repository based on a specification.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	repo := obj.(*api.ImageRepository)
	var stream api.ImageStream
	if err := kapi.Scheme.Convert(repo, &stream); err != nil {
		return nil, err
	}

	createdStream, err := r.imageStreamRegistry.CreateImageStream(ctx, &stream)
	if err != nil {
		return nil, err
	}

	createdRepo := api.ImageRepository{}
	if err := kapi.Scheme.Convert(createdStream, &createdRepo); err != nil {
		return nil, err
	}

	return &createdRepo, nil
}

// Update changes a image repository specification.
func (r *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	repo := obj.(*api.ImageRepository)
	var stream api.ImageStream
	if err := kapi.Scheme.Convert(repo, &stream); err != nil {
		return nil, false, err
	}

	updatedStream, err := r.imageStreamRegistry.UpdateImageStream(ctx, &stream)
	if err != nil {
		return nil, false, err
	}

	updatedRepo := api.ImageRepository{}
	if err := kapi.Scheme.Convert(updatedStream, &updatedRepo); err != nil {
		return nil, false, err
	}
	return &updatedRepo, false, err
}

// Delete deletes an existing image repository specified by its ID.
func (r *REST) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	return r.imageStreamRegistry.DeleteImageStream(ctx, name)
}

// StatusREST implements the REST endpoint for changing the status of an image repository.
type StatusREST struct {
	imageStreamRegistry imagestream.Registry
}

func (r *StatusREST) New() runtime.Object {
	return &api.ImageRepository{}
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	repo := obj.(*api.ImageRepository)
	var stream api.ImageStream
	if err := kapi.Scheme.Convert(&repo, &stream); err != nil {
		return nil, false, err
	}

	updatedStream, err := r.imageStreamRegistry.UpdateImageStreamStatus(ctx, &stream)
	if err != nil {
		return nil, false, err
	}

	updatedRepo := api.ImageRepository{}
	if err := kapi.Scheme.Convert(updatedStream, &updatedRepo); err != nil {
		return nil, false, err
	}
	return &updatedRepo, false, err
}
