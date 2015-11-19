package subjectaccessreview

import (
	api "github.com/openshift/origin/pkg/authorization/api"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
)

type Registry interface {
	CreateSubjectAccessReview(ctx kapi.Context, subjectAccessReview *api.SubjectAccessReview) (*api.SubjectAccessReviewResponse, error)
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

func (s *storage) CreateSubjectAccessReview(ctx kapi.Context, subjectAccessReview *api.SubjectAccessReview) (*api.SubjectAccessReviewResponse, error) {
	obj, err := s.Create(ctx, subjectAccessReview)
	if err != nil {
		return nil, err
	}
	return obj.(*api.SubjectAccessReviewResponse), nil
}
