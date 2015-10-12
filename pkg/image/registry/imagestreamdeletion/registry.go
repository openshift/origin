package imagestreamdeletion

import (
	"github.com/openshift/origin/pkg/image/api"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
)

// Registry is an interface for things that know how to store ImageStreamImageDeletion objects.
type Registry interface {
	ListImageStreamDeletions(ctx kapi.Context, selector labels.Selector) (*api.ImageStreamDeletionList, error)
	GetImageStreamDeletion(ctx kapi.Context, id string) (*api.ImageStreamDeletion, error)
	CreateImageStreamDeletion(ctx kapi.Context, repo *api.ImageStreamDeletion) (*api.ImageStreamDeletion, error)
	DeleteImageStreamDeletion(ctx kapi.Context, id string) (*kapi.Status, error)
}

// Storage is an interface for a standard REST Storage backend
type Storage interface {
	rest.GracefulDeleter
	rest.Lister
	rest.Getter

	Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error)
}

// storage puts strong typing around storage calls
type storage struct {
	Storage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s Storage) Registry {
	return &storage{s}
}

func (s *storage) ListImageStreamDeletions(ctx kapi.Context, label labels.Selector) (*api.ImageStreamDeletionList, error) {
	obj, err := s.List(ctx, label, fields.Everything())
	if err != nil {
		return nil, err
	}
	return obj.(*api.ImageStreamDeletionList), nil
}

func (s *storage) GetImageStreamDeletion(ctx kapi.Context, imageStreamID string) (*api.ImageStreamDeletion, error) {
	obj, err := s.Get(ctx, imageStreamID)
	if err != nil {
		return nil, err
	}
	return obj.(*api.ImageStreamDeletion), nil
}

func (s *storage) CreateImageStreamDeletion(ctx kapi.Context, imageStream *api.ImageStreamDeletion) (*api.ImageStreamDeletion, error) {
	obj, err := s.Create(ctx, imageStream)
	if err != nil {
		return nil, err
	}
	return obj.(*api.ImageStreamDeletion), nil
}

func (s *storage) DeleteImageStreamDeletion(ctx kapi.Context, imageStreamID string) (*kapi.Status, error) {
	obj, err := s.Delete(ctx, imageStreamID, nil)
	if err != nil {
		return nil, err
	}
	return obj.(*kapi.Status), nil
}
