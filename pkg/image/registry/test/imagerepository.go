package test

import (
	"sync"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

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

func (r *ImageRepositoryRegistry) ListImageRepositories(ctx kubeapi.Context, selector labels.Selector) (*api.ImageRepositoryList, error) {
	r.Lock()
	defer r.Unlock()

	return r.ImageRepositories, r.Err
}

func (r *ImageRepositoryRegistry) GetImageRepository(ctx kubeapi.Context, id string) (*api.ImageRepository, error) {
	r.Lock()
	defer r.Unlock()

	return r.ImageRepository, r.Err
}

func (r *ImageRepositoryRegistry) WatchImageRepositories(ctx kubeapi.Context, resourceVersion string, filter func(repo *api.ImageRepository) bool) (watch.Interface, error) {
	return nil, r.Err
}

func (r *ImageRepositoryRegistry) CreateImageRepository(ctx kubeapi.Context, repo *api.ImageRepository) error {
	r.Lock()
	defer r.Unlock()

	r.ImageRepository = repo
	return r.Err
}

func (r *ImageRepositoryRegistry) UpdateImageRepository(ctx kubeapi.Context, repo *api.ImageRepository) error {
	r.Lock()
	defer r.Unlock()

	r.ImageRepository = repo
	return r.Err
}

func (r *ImageRepositoryRegistry) DeleteImageRepository(ctx kubeapi.Context, id string) error {
	r.Lock()
	defer r.Unlock()

	return r.Err
}
