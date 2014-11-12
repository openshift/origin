package image

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewStorage returns a new REST.
func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{registry}
}

// New returns a new Image for use with Create and Update.
func (s *REST) New() runtime.Object {
	return &api.Image{}
}

// List retrieves a list of Images that match selector.
func (s *REST) List(ctx kapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	images, err := s.registry.ListImages(ctx, selector)
	if err != nil {
		return nil, err
	}

	return images, nil
}

// Get retrieves an Image by id.
func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	image, err := s.registry.GetImage(ctx, id)
	if err != nil {
		return nil, err
	}
	return image, nil
}

// Create registers the given Image.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	image, ok := obj.(*api.Image)
	if !ok {
		return nil, fmt.Errorf("not an image: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &image.ObjectMeta) {
		return nil, errors.NewConflict("image", image.Namespace, fmt.Errorf("Image.Namespace does not match the provided context"))
	}

	image.CreationTimestamp = util.Now()

	if errs := validation.ValidateImage(image); len(errs) > 0 {
		return nil, errors.NewInvalid("image", image.Name, errs)
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		if err := s.registry.CreateImage(ctx, image); err != nil {
			return nil, err
		}
		return s.Get(ctx, image.Name)
	}), nil
}

// Update is not supported for Images, as they are immutable.
func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	return nil, fmt.Errorf("Images may not be changed.")
}

// Delete asynchronously deletes an Image specified by its id.
func (s *REST) Delete(ctx kapi.Context, id string) (<-chan apiserver.RESTResult, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return &kapi.Status{Status: kapi.StatusSuccess}, s.registry.DeleteImage(ctx, id)
	}), nil
}

// Watch begins watching for new or deleted Images.
func (r *REST) Watch(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	return r.registry.WatchImages(ctx, label, field, resourceVersion)
}
