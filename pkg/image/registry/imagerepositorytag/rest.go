package imagerepositorytag

import (
	"fmt"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
)

// REST implements the RESTStorage interface for ImageRepositoryTag
// It only supports the Get method and is used to simplify retrieving an Image by tag from an ImageRepository
type REST struct {
	imageRegistry           image.Registry
	imageRepositoryRegistry imagerepository.Registry
}

// NewREST returns a new REST.
func NewREST(imageRegistry image.Registry, imageRepositoryRegistry imagerepository.Registry) apiserver.RESTStorage {
	return &REST{imageRegistry, imageRepositoryRegistry}
}

// New returns a new ImageRepositoryMapping for use with Create.
func (s *REST) New() runtime.Object {
	return &api.ImageRepositoryMapping{}
}

// List is not supported.
func (s *REST) List(ctx kapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	return nil, errors.NewNotFound("imageRepositoryMapping", "")
}

// nameAndTag splits a string into its name component and tag component, and returns an error
// if the string is not in the right form.
func nameAndTag(id string) (name string, tag string, err error) {
	segments := strings.SplitN(id, ":", 2)
	switch len(segments) {
	case 2:
		name = segments[0]
		tag = segments[1]
		if len(name) == 0 || len(tag) == 0 {
			err = errors.NewBadRequest("imageRepositoryTags must be retrieved with <name>:<tag>")
		}
	default:
		err = errors.NewBadRequest("imageRepositoryTags must be retrieved with <name>:<tag>")
	}
	return
}

// Get retrieves images that have been tagged by image and id
func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	name, tag, err := nameAndTag(id)
	if err != nil {
		return nil, err
	}
	repo, err := s.imageRepositoryRegistry.GetImageRepository(ctx, name)
	if err != nil {
		return nil, err
	}
	if repo.Tags == nil {
		return nil, errors.NewNotFound("imageRepositoryTag", tag)
	}
	imageName, ok := repo.Tags[tag]
	if !ok {
		return nil, errors.NewNotFound("imageRepositoryTag", tag)
	}
	return s.imageRegistry.GetImage(ctx, imageName)
}

// Create is not supported.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	return nil, errors.NewNotFound("imageRepositoryMapping", "")
}

// Update is not supported.
func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	return nil, fmt.Errorf("ImageRepositoryTags may not be changed.")
}

// Delete is not supported.
func (s *REST) Delete(ctx kapi.Context, id string) (<-chan apiserver.RESTResult, error) {
	return nil, errors.NewNotFound("imageRepositoryMapping", id)
}
