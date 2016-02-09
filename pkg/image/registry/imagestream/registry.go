package imagestream

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/image/api"
)

// Registry is an interface for things that know how to store ImageStream objects.
type Registry interface {
	// ListImageStreams obtains a list of image streams that match a selector.
	ListImageStreams(ctx kapi.Context, options *kapi.ListOptions) (*api.ImageStreamList, error)
	// GetImageStream retrieves a specific image stream.
	GetImageStream(ctx kapi.Context, id string) (*api.ImageStream, error)
	// CreateImageStream creates a new image stream.
	CreateImageStream(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error)
	// UpdateImageStream updates an image stream.
	UpdateImageStream(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error)
	// UpdateImageStreamSpec updates an image stream's spec.
	UpdateImageStreamSpec(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error)
	// UpdateImageStreamStatus updates an image stream's status.
	UpdateImageStreamStatus(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error)
	// DeleteImageStream deletes an image stream.
	DeleteImageStream(ctx kapi.Context, id string) (*unversioned.Status, error)
	// WatchImageStreams watches for new/changed/deleted image streams.
	WatchImageStreams(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error)
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
	status   rest.Updater
	internal rest.Updater
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s Storage, status, internal rest.Updater) Registry {
	return &storage{Storage: s, status: status, internal: internal}
}

func (s *storage) ListImageStreams(ctx kapi.Context, options *kapi.ListOptions) (*api.ImageStreamList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*api.ImageStreamList), nil
}

func (s *storage) GetImageStream(ctx kapi.Context, imageStreamID string) (*api.ImageStream, error) {
	obj, err := s.Get(ctx, imageStreamID)
	if err != nil {
		return nil, err
	}
	return obj.(*api.ImageStream), nil
}

func (s *storage) CreateImageStream(ctx kapi.Context, imageStream *api.ImageStream) (*api.ImageStream, error) {
	obj, err := s.Create(ctx, imageStream)
	if err != nil {
		return nil, err
	}
	return obj.(*api.ImageStream), nil
}

func (s *storage) UpdateImageStream(ctx kapi.Context, imageStream *api.ImageStream) (*api.ImageStream, error) {
	obj, _, err := s.internal.Update(ctx, imageStream)
	if err != nil {
		return nil, err
	}
	return obj.(*api.ImageStream), nil
}

func (s *storage) UpdateImageStreamSpec(ctx kapi.Context, imageStream *api.ImageStream) (*api.ImageStream, error) {
	obj, _, err := s.Update(ctx, imageStream)
	if err != nil {
		return nil, err
	}
	return obj.(*api.ImageStream), nil
}

func (s *storage) UpdateImageStreamStatus(ctx kapi.Context, imageStream *api.ImageStream) (*api.ImageStream, error) {
	obj, _, err := s.status.Update(ctx, imageStream)
	if err != nil {
		return nil, err
	}
	return obj.(*api.ImageStream), nil
}

func (s *storage) DeleteImageStream(ctx kapi.Context, imageStreamID string) (*unversioned.Status, error) {
	obj, err := s.Delete(ctx, imageStreamID, nil)
	if err != nil {
		return nil, err
	}
	return obj.(*unversioned.Status), nil
}

func (s *storage) WatchImageStreams(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}
