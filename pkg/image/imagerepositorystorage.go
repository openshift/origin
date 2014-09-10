package image

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"

	baseapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/openshift/origin/pkg/image/api"
)

// ImageRepositoryStorage implements the RESTStorage interface in terms of an ImageRepositoryRegistry.
type ImageRepositoryStorage struct {
	registry ImageRepositoryRegistry
}

// NewImageRepositoryStorage returns a new ImageRepositoryStorage.
func NewImageRepositoryStorage(registry ImageRepositoryRegistry) apiserver.RESTStorage {
	return &ImageRepositoryStorage{registry}
}

// New returns a new ImageRepository for use with Create and Update.
func (s *ImageRepositoryStorage) New() interface{} {
	return &api.ImageRepository{}
}

// Get retrieves an ImageRepository by id.
func (s *ImageRepositoryStorage) Get(id string) (interface{}, error) {
	repo, err := s.registry.GetImageRepository(id)
	if err != nil {
		return nil, err
	}
	return repo, nil
}

// List retrieves a list of ImageRepositories that match selector.
func (s *ImageRepositoryStorage) List(selector labels.Selector) (interface{}, error) {
	imageRepositories, err := s.registry.ListImageRepositories(selector)
	if err != nil {
		return nil, err
	}
	return imageRepositories, err
}

// Watch begins watching for new, changed, or deleted ImageRepositories.
func (s *ImageRepositoryStorage) Watch(label, field labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	return s.registry.WatchImageRepositories(resourceVersion, func(repo *api.ImageRepository) bool {
		fields := labels.Set{
			"ID": repo.ID,
			"DockerImageRepository": repo.DockerImageRepository,
		}
		return label.Matches(labels.Set(repo.Labels)) && field.Matches(fields)
	})
}

// Create registers the given ImageRepository.
func (s *ImageRepositoryStorage) Create(obj interface{}) (<-chan interface{}, error) {
	repo, ok := obj.(*api.ImageRepository)
	if !ok {
		return nil, fmt.Errorf("not an image repository: %#v", obj)
	}

	if len(repo.ID) == 0 {
		repo.ID = uuid.NewUUID().String()
	}

	if repo.Tags == nil {
		repo.Tags = make(map[string]string)
	}

	repo.CreationTimestamp = util.Now()

	return apiserver.MakeAsync(func() (interface{}, error) {
		if err := s.registry.CreateImageRepository(repo); err != nil {
			return nil, err
		}
		return s.Get(repo.ID)
	}), nil
}

// Update replaces an existing ImageRepository in the registry with the given ImageRepository.
func (s *ImageRepositoryStorage) Update(obj interface{}) (<-chan interface{}, error) {
	repo, ok := obj.(*api.ImageRepository)
	if !ok {
		return nil, fmt.Errorf("not an image repository: %#v", obj)
	}
	if len(repo.ID) == 0 {
		return nil, fmt.Errorf("id is unspecified: %#v", repo)
	}

	return apiserver.MakeAsync(func() (interface{}, error) {
		err := s.registry.UpdateImageRepository(repo)
		if err != nil {
			return nil, err
		}
		return s.Get(repo.ID)
	}), nil
}

// Delete asynchronously deletes an ImageRepository specified by its id.
func (s *ImageRepositoryStorage) Delete(id string) (<-chan interface{}, error) {
	return apiserver.MakeAsync(func() (interface{}, error) {
		return &baseapi.Status{Status: baseapi.StatusSuccess}, s.registry.DeleteImageRepository(id)
	}), nil
}
