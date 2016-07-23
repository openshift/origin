package image

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/image/api"
)

// Registry is an interface for things that know how to store Image objects.
type Registry interface {
	// ListImages obtains a list of images that match a selector.
	ListImages(ctx kapi.Context, options *kapi.ListOptions) (*api.ImageList, error)
	// GetImage retrieves a specific image.
	GetImage(ctx kapi.Context, id string) (*api.Image, error)
	// CreateImage creates a new image.
	CreateImage(ctx kapi.Context, image *api.Image) error
	// DeleteImage deletes an image.
	DeleteImage(ctx kapi.Context, id string) error
	// WatchImages watches for new or deleted images.
	WatchImages(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error)
	// UpdateImage updates given image.
	UpdateImage(ctx kapi.Context, image *api.Image) (*api.Image, error)
}

// Storage is an interface for a standard REST Storage backend
type Storage interface {
	rest.GracefulDeleter
	rest.Lister
	rest.Getter
	rest.Watcher

	Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error)
	Update(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error)
}

// storage puts strong typing around storage calls
type storage struct {
	Storage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s Storage) Registry {
	return &storage{Storage: s}
}

func (s *storage) ListImages(ctx kapi.Context, options *kapi.ListOptions) (*api.ImageList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*api.ImageList), nil
}

func (s *storage) GetImage(ctx kapi.Context, imageID string) (*api.Image, error) {
	obj, err := s.Get(ctx, imageID)
	if err != nil {
		return nil, err
	}
	return obj.(*api.Image), nil
}

func (s *storage) CreateImage(ctx kapi.Context, image *api.Image) error {
	_, err := s.Create(ctx, image)
	return err
}

func (s *storage) UpdateImage(ctx kapi.Context, image *api.Image) (*api.Image, error) {
	obj, _, err := s.Update(ctx, image.Name, rest.DefaultUpdatedObjectInfo(image, kapi.Scheme))
	if err != nil {
		return nil, err
	}
	return obj.(*api.Image), nil
}

func (s *storage) DeleteImage(ctx kapi.Context, imageID string) error {
	_, err := s.Delete(ctx, imageID, nil)
	return err
}

func (s *storage) WatchImages(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}
