package podspecsubjectreview

import (
	api "github.com/openshift/origin/pkg/security/api"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
)

type Registry interface {
	CreatePodSpecSubjectReview(ctx kapi.Context, podSpecSubjectReview *api.PodSpecSubjectReview) (*api.PodSpecSubjectReview, error)
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

func (s *storage) CreatePodSpecSubjectReview(ctx kapi.Context, podSpecSubjectReview *api.PodSpecSubjectReview) (*api.PodSpecSubjectReview, error) {
	obj, err := s.Create(ctx, podSpecSubjectReview)
	if err != nil {
		return nil, err
	}
	return obj.(*api.PodSpecSubjectReview), nil
}
