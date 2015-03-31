package imagerepository

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/image/api"
)

// Registry is an interface for things that know how to store ImageRepository objects.
type Registry interface {
	// ListImageRepositories obtains a list of image repositories that match a selector.
	ListImageRepositories(ctx kapi.Context, selector labels.Selector) (*api.ImageRepositoryList, error)
	// GetImageRepository retrieves a specific image repository.
	GetImageRepository(ctx kapi.Context, id string) (*api.ImageRepository, error)
	// CreateImageRepository creates a new image repository.
	CreateImageRepository(ctx kapi.Context, repo *api.ImageRepository) error
	// UpdateImageRepository updates an image repository.
	UpdateImageRepository(ctx kapi.Context, repo *api.ImageRepository) error
	// UpdateImageRepository updates an image repository's status.
	UpdateImageRepositoryStatus(ctx kapi.Context, repo *api.ImageRepository) error
	// DeleteImageRepository deletes an image repository.
	DeleteImageRepository(ctx kapi.Context, id string) error
	// WatchImageRepositories watches for new/changed/deleted image repositories.
	WatchImageRepositories(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
}

// Storage is an interface for a standard REST Storage backend
type Storage interface {
	rest.GracefulDeleter
	rest.Lister
	rest.Getter
	rest.Watcher

	Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error)
	Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error)
}

// storage puts strong typing around storage calls
type storage struct {
	Storage
	status rest.Updater
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s Storage, status rest.Updater) Registry {
	return &storage{s, status}
}

func (s *storage) ListImageRepositories(ctx kapi.Context, label labels.Selector) (*api.ImageRepositoryList, error) {
	obj, err := s.List(ctx, label, fields.Everything())
	if err != nil {
		return nil, err
	}
	return obj.(*api.ImageRepositoryList), nil
}

func (s *storage) GetImageRepository(ctx kapi.Context, imageRepositoryID string) (*api.ImageRepository, error) {
	obj, err := s.Get(ctx, imageRepositoryID)
	if err != nil {
		return nil, err
	}
	return obj.(*api.ImageRepository), nil
}

func (s *storage) CreateImageRepository(ctx kapi.Context, imageRepository *api.ImageRepository) error {
	_, err := s.Create(ctx, imageRepository)
	return err
}

func (s *storage) UpdateImageRepository(ctx kapi.Context, imageRepository *api.ImageRepository) error {
	_, _, err := s.Update(ctx, imageRepository)
	return err
}

func (s *storage) UpdateImageRepositoryStatus(ctx kapi.Context, imageRepository *api.ImageRepository) error {
	_, _, err := s.status.Update(ctx, imageRepository)
	return err
}

func (s *storage) DeleteImageRepository(ctx kapi.Context, imageRepositoryID string) error {
	_, err := s.Delete(ctx, imageRepositoryID, nil)
	return err
}

func (s *storage) WatchImageRepositories(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return s.Watch(ctx, label, field, resourceVersion)
}
