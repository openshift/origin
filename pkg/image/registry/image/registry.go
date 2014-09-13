package image

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/image/api"
)

// Registry is an interface for things that know how to store Image objects.
type Registry interface {
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
