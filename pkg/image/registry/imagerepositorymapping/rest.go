package imagerepositorymapping

import (
	"fmt"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
)

// REST implements the RESTStorage interface in terms of an Registry and Registry.
// It Only supports the Create method and is used to simply adding a new Image and tag to an ImageRepository.
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
func (s *REST) List(ctx kubeapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	return nil, errors.NewNotFound("imageRepositoryMapping", "list")
}

// Get is not supported.
func (s *REST) Get(ctx kubeapi.Context, id string) (runtime.Object, error) {
	return nil, errors.NewNotFound("imageRepositoryMapping", id)
}

// Create registers a new image (if it doesn't exist) and updates the specified ImageRepository's tags.
func (s *REST) Create(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	mapping, ok := obj.(*api.ImageRepositoryMapping)
	if !ok {
		return nil, fmt.Errorf("not an image repository mapping: %#v", obj)
	}

	repo, err := s.findImageRepository(mapping.DockerImageRepository)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, errors.NewInvalid("imageRepositoryMapping", mapping.ID, errors.ErrorList{
			errors.NewFieldNotFound("DockerImageRepository", mapping.DockerImageRepository),
		})
	}

	if errs := validation.ValidateImageRepositoryMapping(mapping); len(errs) > 0 {
		return nil, errors.NewInvalid("imageRepositoryMapping", mapping.ID, errs)
	}

	image := mapping.Image

	image.CreationTimestamp = util.Now()

	//TODO apply metadata overrides

	if repo.Tags == nil {
		repo.Tags = make(map[string]string)
	}
	repo.Tags[mapping.Tag] = image.ID

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err = s.imageRegistry.CreateImage(&image)
		if err != nil && !errors.IsAlreadyExists(err) {
			return nil, err
		}

		err = s.imageRepositoryRegistry.UpdateImageRepository(repo)
		if err != nil {
			return nil, err
		}

		return &kubeapi.Status{Status: kubeapi.StatusSuccess}, nil
	}), nil
}

// findImageRepository retrieves an ImageRepository whose DockerImageRepository matches dockerRepo.
func (s *REST) findImageRepository(dockerRepo string) (*api.ImageRepository, error) {
	//TODO make this more efficient
	list, err := s.imageRepositoryRegistry.ListImageRepositories(labels.Everything())
	if err != nil {
		return nil, err
	}

	var repo *api.ImageRepository
	for _, r := range list.Items {
		if dockerRepo == r.DockerImageRepository {
			repo = &r
			break
		}
	}

	return repo, nil
}

// Update is not supported.
func (s *REST) Update(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	return nil, fmt.Errorf("ImageRepositoryMappings may not be changed.")
}

// Delete is not supported.
func (s *REST) Delete(ctx kubeapi.Context, id string) (<-chan runtime.Object, error) {
	return nil, errors.NewNotFound("imageRepositoryMapping", id)
}
