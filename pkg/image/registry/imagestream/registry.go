package imagestream

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// Registry is an interface for things that know how to store ImageStream objects.
type Registry interface {
	// ListImageStreams obtains a list of image streams that match a selector.
	ListImageStreams(ctx apirequest.Context, options *metainternal.ListOptions) (*imageapi.ImageStreamList, error)
	// GetImageStream retrieves a specific image stream.
	GetImageStream(ctx apirequest.Context, id string, options *metav1.GetOptions) (*imageapi.ImageStream, error)
	// CreateImageStream creates a new image stream.
	CreateImageStream(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error)
	// UpdateImageStream updates an image stream.
	UpdateImageStream(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error)
	// UpdateImageStreamSpec updates an image stream's spec.
	UpdateImageStreamSpec(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error)
	// UpdateImageStreamStatus updates an image stream's status.
	UpdateImageStreamStatus(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error)
	// DeleteImageStream deletes an image stream.
	DeleteImageStream(ctx apirequest.Context, id string) (*metav1.Status, error)
	// WatchImageStreams watches for new/changed/deleted image streams.
	WatchImageStreams(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error)
}

// Storage is an interface for a standard REST Storage backend
type Storage interface {
	rest.GracefulDeleter
	rest.Lister
	rest.Getter
	rest.Watcher

	Create(ctx apirequest.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, _ bool) (runtime.Object, error)
	Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (runtime.Object, bool, error)
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

func (s *storage) ListImageStreams(ctx apirequest.Context, options *metainternal.ListOptions) (*imageapi.ImageStreamList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*imageapi.ImageStreamList), nil
}

func (s *storage) GetImageStream(ctx apirequest.Context, imageStreamID string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
	obj, err := s.Get(ctx, imageStreamID, options)
	if err != nil {
		return nil, err
	}
	return obj.(*imageapi.ImageStream), nil
}

func (s *storage) CreateImageStream(ctx apirequest.Context, imageStream *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	obj, err := s.Create(ctx, imageStream, rest.ValidateAllObjectFunc, false)
	if err != nil {
		return nil, err
	}
	return obj.(*imageapi.ImageStream), nil
}

func (s *storage) UpdateImageStream(ctx apirequest.Context, imageStream *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	obj, _, err := s.internal.Update(ctx, imageStream.Name, rest.DefaultUpdatedObjectInfo(imageStream), rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc)
	if err != nil {
		return nil, err
	}
	return obj.(*imageapi.ImageStream), nil
}

func (s *storage) UpdateImageStreamSpec(ctx apirequest.Context, imageStream *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	obj, _, err := s.Update(ctx, imageStream.Name, rest.DefaultUpdatedObjectInfo(imageStream), rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc)
	if err != nil {
		return nil, err
	}
	return obj.(*imageapi.ImageStream), nil
}

func (s *storage) UpdateImageStreamStatus(ctx apirequest.Context, imageStream *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	obj, _, err := s.status.Update(ctx, imageStream.Name, rest.DefaultUpdatedObjectInfo(imageStream), rest.ValidateAllObjectFunc, rest.ValidateAllObjectUpdateFunc)
	if err != nil {
		return nil, err
	}
	return obj.(*imageapi.ImageStream), nil
}

func (s *storage) DeleteImageStream(ctx apirequest.Context, imageStreamID string) (*metav1.Status, error) {
	obj, _, err := s.Delete(ctx, imageStreamID, nil)
	if err != nil {
		return nil, err
	}
	return obj.(*metav1.Status), nil
}

func (s *storage) WatchImageStreams(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}
