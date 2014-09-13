package test

import (
	"sync"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/openshift/origin/pkg/image/api"
)

type ImageRepositoryRegistry struct {
	Err               error
	ImageRepository   *api.ImageRepository
	ImageRepositories *api.ImageRepositoryList
	sync.Mutex
}

func NewImageRepositoryRegistry() *ImageRepositoryRegistry {
	return &ImageRepositoryRegistry{}
}

func (r *ImageRepositoryRegistry) ListImageRepositories(selector labels.Selector) (*api.ImageRepositoryList, error) {
	r.Lock()
	defer r.Unlock()

	return r.ImageRepositories, r.Err
}

func (r *ImageRepositoryRegistry) GetImageRepository(id string) (*api.ImageRepository, error) {
	r.Lock()
	defer r.Unlock()

	return r.ImageRepository, r.Err
}

func (r *ImageRepositoryRegistry) WatchImageRepositories(resourceVersion uint64, filter func(repo *api.ImageRepository) bool) (watch.Interface, error) {
	return nil, r.Err
}

func (r *ImageRepositoryRegistry) CreateImageRepository(repo *api.ImageRepository) error {
	r.Lock()
	defer r.Unlock()

	r.ImageRepository = repo
	return r.Err
}

func (r *ImageRepositoryRegistry) UpdateImageRepository(repo *api.ImageRepository) error {
	r.Lock()
	defer r.Unlock()

	r.ImageRepository = repo
	return r.Err
}

func (r *ImageRepositoryRegistry) DeleteImageRepository(id string) error {
	r.Lock()
	defer r.Unlock()

	return r.Err
}
