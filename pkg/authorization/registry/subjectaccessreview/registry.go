package subjectaccessreview

import (
	api "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
)

type Registry interface {
	CreateSubjectAccessReview(ctx apirequest.Context, subjectAccessReview *api.SubjectAccessReview) (*api.SubjectAccessReviewResponse, error)
}

type Storage interface {
	Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error)
}

type storage struct {
	Storage
}

func NewRegistry(s Storage) Registry {
	return &storage{s}
}

func (s *storage) CreateSubjectAccessReview(ctx apirequest.Context, subjectAccessReview *api.SubjectAccessReview) (*api.SubjectAccessReviewResponse, error) {
	obj, err := s.Create(ctx, subjectAccessReview, false)
	if err != nil {
		return nil, err
	}
	return obj.(*api.SubjectAccessReviewResponse), nil
}
