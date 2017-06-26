package imagestreamtag

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// Registry is an interface for things that know how to store ImageStreamTag objects.
type Registry interface {
	GetImageStreamTag(ctx apirequest.Context, nameAndTag string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error)
	DeleteImageStreamTag(ctx apirequest.Context, nameAndTag string) (*metav1.Status, error)
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

func (s *storage) GetImageStreamTag(ctx apirequest.Context, nameAndTag string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error) {
	obj, err := s.Get(ctx, nameAndTag, options)
	if err != nil {
		return nil, err
	}
	return obj.(*imageapi.ImageStreamTag), nil
}

func (s *storage) DeleteImageStreamTag(ctx apirequest.Context, nameAndTag string) (*metav1.Status, error) {
	obj, err := s.Delete(ctx, nameAndTag)
	if err != nil {
		return nil, err
	}
	return obj.(*metav1.Status), err
}
