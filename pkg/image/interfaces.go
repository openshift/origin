package image

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
)
import "github.com/openshift/origin/pkg/image/api"

// ImageRegistry is an interface for things that know how to store Image objects.
type ImageRegistry interface {
	// ListImages obtains a list of images that match a selector.
	ListImages(selector labels.Selector) (*api.ImageList, error)
	// GetImage retrieves a specific image.
	GetImage(id string) (*api.Image, error)
	// CreateImage creates a new image.
	CreateImage(image *api.Image) error
	// UpdateImage updates an image.
	UpdateImage(image *api.Image) error
	// DeleteImage deletes an image.
	DeleteImage(id string) error
}

// ImageRepositoryRegistry is an interface for things that know how to store ImageRepository objects.
type ImageRepositoryRegistry interface {
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
