package image

import (
	"fmt"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
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
func (s *REST) New() interface{} {
	return &api.Image{}
}

// Get retrieves an Image by id.
func (s *REST) Get(id string) (interface{}, error) {
	image, err := s.registry.GetImage(id)
	if err != nil {
		return nil, err
	}
	return image, nil
}

// List retrieves a list of Images that match selector.
func (s *REST) List(selector labels.Selector) (interface{}, error) {
	images, err := s.registry.ListImages(selector)
	if err != nil {
		return nil, err
	}

	return images, nil
}

// Create registers the given Image.
func (s *REST) Create(obj interface{}) (<-chan interface{}, error) {
	image, ok := obj.(*api.Image)
	if !ok {
		return nil, fmt.Errorf("not an image: %#v", obj)
	}

	image.CreationTimestamp = util.Now()

	if errs := validation.ValidateImage(image); len(errs) > 0 {
		return nil, errors.NewInvalid("image", image.ID, errs)
	}

	return apiserver.MakeAsync(func() (interface{}, error) {
		if err := s.registry.CreateImage(image); err != nil {
			return nil, err
		}
		return s.Get(image.ID)
	}), nil
}

// Update is not supported for Images, as they are immutable.
func (s *REST) Update(obj interface{}) (<-chan interface{}, error) {
	return nil, fmt.Errorf("Images may not be changed.")
}

// Delete asynchronously deletes an Image specified by its id.
func (s *REST) Delete(id string) (<-chan interface{}, error) {
	return apiserver.MakeAsync(func() (interface{}, error) {
		return &kubeapi.Status{Status: kubeapi.StatusSuccess}, s.registry.DeleteImage(id)
	}), nil
}
