package image

import (
	"fmt"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/openshift/origin/pkg/image/api"
)

// ImageRepositoryMappingStorage implements the RESTStorage interface in terms of an ImageRegistry and ImageRepositoryRegistry.
// It Only supports the Create method and is used to simply adding a new Image and tag to an ImageRepository.
type ImageRepositoryMappingStorage struct {
	imageRegistry           ImageRegistry
	imageRepositoryRegistry ImageRepositoryRegistry
}

// NewImageRepositoryMappingStorage returns a new ImageRepositoryMappingStorage.
func NewImageRepositoryMappingStorage(imageRegistry ImageRegistry, imageRepositoryRegistry ImageRepositoryRegistry) apiserver.RESTStorage {
	return &ImageRepositoryMappingStorage{imageRegistry, imageRepositoryRegistry}
}

// New returns a new ImageRepositoryMapping for use with Create.
func (s *ImageRepositoryMappingStorage) New() interface{} {
	return &api.ImageRepositoryMapping{}
}

// Get is not supported.
func (s *ImageRepositoryMappingStorage) Get(id string) (interface{}, error) {
	return nil, errors.NewNotFound("imageRepositoryMapping", id)
}

// List is not supported.
func (s *ImageRepositoryMappingStorage) List(selector labels.Selector) (interface{}, error) {
	return nil, errors.NewNotFound("imageRepositoryMapping", "list")
}

// Create registers a new image (if it doesn't exist) and updates the specified ImageRepository's tags.
func (s *ImageRepositoryMappingStorage) Create(obj interface{}) (<-chan interface{}, error) {
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

	if errs := ValidateImageRepositoryMapping(mapping); len(errs) > 0 {
		return nil, errors.NewInvalid("imageRepositoryMapping", mapping.ID, errs)
	}

	image := mapping.Image

	image.CreationTimestamp = util.Now()

	//TODO apply metadata overrides

	if repo.Tags == nil {
		repo.Tags = make(map[string]string)
	}
	repo.Tags[mapping.Tag] = image.ID

	return apiserver.MakeAsync(func() (interface{}, error) {
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
func (s *ImageRepositoryMappingStorage) findImageRepository(dockerRepo string) (*api.ImageRepository, error) {
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
func (s *ImageRepositoryMappingStorage) Update(obj interface{}) (<-chan interface{}, error) {
	return nil, fmt.Errorf("ImageRepositoryMappings may not be changed.")
}

// Delete is not supported.
func (s *ImageRepositoryMappingStorage) Delete(id string) (<-chan interface{}, error) {
	return nil, errors.NewNotFound("imageRepositoryMapping", id)
}
