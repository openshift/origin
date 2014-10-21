package image

import (
	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/image/api"
)

// Registry is an interface for things that know how to store Image objects.
type Registry interface {
	// ListImages obtains a list of images that match a selector.
	ListImages(ctx kubeapi.Context, selector labels.Selector) (*api.ImageList, error)
	// GetImage retrieves a specific image.
	GetImage(ctx kubeapi.Context, id string) (*api.Image, error)
	// CreateImage creates a new image.
	CreateImage(ctx kubeapi.Context, image *api.Image) error
	// UpdateImage updates an image.
	UpdateImage(ctx kubeapi.Context, image *api.Image) error
	// DeleteImage deletes an image.
	DeleteImage(ctx kubeapi.Context, id string) error
}
