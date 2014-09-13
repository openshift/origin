package imagerepository

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/openshift/origin/pkg/image/api"
)

// Registry is an interface for things that know how to store ImageRepository objects.
type Registry interface {
	// ListImageRepositories obtains a list of image repositories that match a selector.
	ListImageRepositories(selector labels.Selector) (*api.ImageRepositoryList, error)
	// GetImageRepository retrieves a specific image repository.
	GetImageRepository(id string) (*api.ImageRepository, error)
	// WatchImageRepositories watches for new/changed/deleted image repositories.
	WatchImageRepositories(resourceVersion uint64, filter func(repo *api.ImageRepository) bool) (watch.Interface, error)
	// CreateImageRepository creates a new image repository.
	CreateImageRepository(repo *api.ImageRepository) error
	// UpdateImageRepository updates an image repository.
	UpdateImageRepository(repo *api.ImageRepository) error
	// DeleteImageRepository deletes an image repository.
	DeleteImageRepository(id string) error
}
