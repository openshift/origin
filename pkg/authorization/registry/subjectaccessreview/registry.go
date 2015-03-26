package subjectaccessreview

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	api "github.com/openshift/origin/pkg/authorization/api"
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
