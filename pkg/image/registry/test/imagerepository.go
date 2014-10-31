package test

import (
	"sync"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

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

func (r *ImageRepositoryRegistry) ListImageRepositories(ctx kapi.Context, selector labels.Selector) (*api.ImageRepositoryList, error) {
	r.Lock()
	defer r.Unlock()

	return r.ImageRepositories, r.Err
}

func (r *ImageRepositoryRegistry) GetImageRepository(ctx kapi.Context, id string) (*api.ImageRepository, error) {
	r.Lock()
	defer r.Unlock()

	return r.ImageRepository, r.Err
}

func (r *ImageRepositoryRegistry) WatchImageRepositories(ctx kapi.Context, resourceVersion string, filter func(repo *api.ImageRepository) bool) (watch.Interface, error) {
	return nil, r.Err
}

func (r *ImageRepositoryRegistry) CreateImageRepository(ctx kapi.Context, repo *api.ImageRepository) error {
	r.Lock()
	defer r.Unlock()

	r.ImageRepository = repo
	return r.Err
}

func (r *ImageRepositoryRegistry) UpdateImageRepository(ctx kapi.Context, repo *api.ImageRepository) error {
	r.Lock()
	defer r.Unlock()

	r.ImageRepository = repo
	return r.Err
}

func (r *ImageRepositoryRegistry) DeleteImageRepository(ctx kapi.Context, id string) error {
	r.Lock()
	defer r.Unlock()

	return r.Err
}
