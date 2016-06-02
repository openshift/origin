package podspecselfsubjectreview

import (
	api "github.com/openshift/origin/pkg/security/api"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
)

type Registry interface {
	CreatePodSpecSelfSubjectReview(ctx kapi.Context, podSpecReview *api.PodSpecSelfSubjectReview) (*api.PodSpecSelfSubjectReview, error)
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

func (s *storage) CreatePodSpecSelfSubjectReview(ctx kapi.Context, podSpecReview *api.PodSpecSelfSubjectReview) (*api.PodSpecSelfSubjectReview, error) {
	obj, err := s.Create(ctx, podSpecReview)
	if err != nil {
		return nil, err
	}
	return obj.(*api.PodSpecSelfSubjectReview), nil
}
