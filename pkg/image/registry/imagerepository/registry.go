package imagerepository

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/openshift/origin/pkg/image/api"
)

// Registry is an interface for things that know how to store ImageRepository objects.
type Registry interface {
	// ListImageRepositories obtains a list of image repositories that match a selector.
	ListImageRepositories(ctx kapi.Context, selector labels.Selector) (*api.ImageRepositoryList, error)
	// GetImageRepository retrieves a specific image repository.
	GetImageRepository(ctx kapi.Context, id string) (*api.ImageRepository, error)
	// WatchImageRepositories watches for new/changed/deleted image repositories.
	WatchImageRepositories(ctx kapi.Context, resourceVersion string, filter func(repo *api.ImageRepository) bool) (watch.Interface, error)
	// CreateImageRepository creates a new image repository.
	CreateImageRepository(ctx kapi.Context, repo *api.ImageRepository) error
	// UpdateImageRepository updates an image repository.
	UpdateImageRepository(ctx kapi.Context, repo *api.ImageRepository) error
	// DeleteImageRepository deletes an image repository.
	DeleteImageRepository(ctx kapi.Context, id string) error
}
