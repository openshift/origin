package imagestreamtag

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	"github.com/openshift/origin/pkg/image/api"
)

// Registry is an interface for things that know how to store ImageStreamTag objects.
type Registry interface {
	GetImageStreamTag(ctx kapi.Context, nameAndTag string) (*api.Image, error)
	DeleteImageStreamTag(ctx kapi.Context, nameAndTag string) (*kapi.Status, error)
}

// Storage is an interface for a standard REST Storage backend
type Storage interface {
	rest.Deleter
	rest.Getter
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

func (s *storage) GetImageStreamTag(ctx kapi.Context, nameAndTag string) (*api.Image, error) {
	obj, err := s.Get(ctx, nameAndTag)
	if err != nil {
		return nil, err
	}
	return obj.(*api.Image), nil
}

func (s *storage) DeleteImageStreamTag(ctx kapi.Context, nameAndTag string) (*kapi.Status, error) {
	obj, err := s.Delete(ctx, nameAndTag)
	if err != nil {
		return nil, err
	}
	return obj.(*kapi.Status), err
}
