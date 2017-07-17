package image

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// Registry is an interface for things that know how to store Image objects.
type Registry interface {
	// ListImages obtains a list of images that match a selector.
	ListImages(ctx apirequest.Context, options *metainternal.ListOptions) (*imageapi.ImageList, error)
	// GetImage retrieves a specific image.
	GetImage(ctx apirequest.Context, id string, options *metav1.GetOptions) (*imageapi.Image, error)
	// CreateImage creates a new image.
	CreateImage(ctx apirequest.Context, image *imageapi.Image) error
	// DeleteImage deletes an image.
	DeleteImage(ctx apirequest.Context, id string) error
	// WatchImages watches for new or deleted images.
	WatchImages(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error)
	// UpdateImage updates given image.
	UpdateImage(ctx apirequest.Context, image *imageapi.Image) (*imageapi.Image, error)
}

// Storage is an interface for a standard REST Storage backend
type Storage interface {
	rest.GracefulDeleter
	rest.Lister
	rest.Getter
	rest.Watcher

	Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error)
	Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error)
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

func (s *storage) ListImages(ctx apirequest.Context, options *metainternal.ListOptions) (*imageapi.ImageList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*imageapi.ImageList), nil
}

func (s *storage) GetImage(ctx apirequest.Context, imageID string, options *metav1.GetOptions) (*imageapi.Image, error) {
	obj, err := s.Get(ctx, imageID, options)
	if err != nil {
		return nil, err
	}
	return obj.(*imageapi.Image), nil
}

func (s *storage) CreateImage(ctx apirequest.Context, image *imageapi.Image) error {
	_, err := s.Create(ctx, image, false)
	return err
}

func (s *storage) UpdateImage(ctx apirequest.Context, image *imageapi.Image) (*imageapi.Image, error) {
	obj, _, err := s.Update(ctx, image.Name, rest.DefaultUpdatedObjectInfo(image, kapi.Scheme))
	if err != nil {
		return nil, err
	}
	return obj.(*imageapi.Image), nil
}

func (s *storage) DeleteImage(ctx apirequest.Context, imageID string) error {
	_, _, err := s.Delete(ctx, imageID, nil)
	return err
}

func (s *storage) WatchImages(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}
