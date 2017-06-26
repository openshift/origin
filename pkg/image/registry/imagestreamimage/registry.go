package imagestreamimage

import (
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

// Registry is an interface for things that know how to store ImageStreamImage objects.
type Registry interface {
	GetImageStreamImage(ctx apirequest.Context, nameAndTag string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error)
}

// Storage is an interface for a standard REST Storage backend
type Storage interface {
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

func (s *storage) GetImageStreamImage(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*imageapi.ImageStreamImage), nil
}
