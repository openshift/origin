package image

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/openshift/origin/pkg/image/api"
)

// Registry is an interface for things that know how to store Image objects.
type Registry interface {
	// ListImages obtains a list of images that match a selector.
	ListImages(ctx kapi.Context, selector labels.Selector) (*api.ImageList, error)
	// GetImage retrieves a specific image.
	GetImage(ctx kapi.Context, id string) (*api.Image, error)
	// CreateImage creates a new image.
	CreateImage(ctx kapi.Context, image *api.Image) error
	// UpdateImage updates an image.
	UpdateImage(ctx kapi.Context, image *api.Image) error
	// DeleteImage deletes an image.
	DeleteImage(ctx kapi.Context, id string) error
	// WatchImages watches for new or deleted images.
	WatchImages(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error)
}
