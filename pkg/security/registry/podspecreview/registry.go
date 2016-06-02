package podspecreview

import (
	api "github.com/openshift/origin/pkg/security/api"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
)

type Registry interface {
	CreatePodSpecReview(ctx kapi.Context, podSpecReview *api.PodSpecReview) (*api.PodSpecReview, error)
}

type Storage interface {
	Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error)
}

type storage struct {
	Storage
}

func NewRegistry(s Storage) Registry {
	return &storage{s}
}

func (s *storage) CreatePodSpecReview(ctx kapi.Context, podSpecReview *api.PodSpecReview) (*api.PodSpecReview, error) {
	obj, err := s.Create(ctx, podSpecReview)
	if err != nil {
		return nil, err
	}
	return obj.(*api.PodSpecReview), nil
}
