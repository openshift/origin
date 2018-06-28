package subjectaccessreview

import (
	"context"

	api "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

type Registry interface {
	CreateSubjectAccessReview(ctx context.Context, subjectAccessReview *api.SubjectAccessReview) (*api.SubjectAccessReviewResponse, error)
}

type Storage interface {
	Create(ctx context.Context, obj runtime.Object, _ rest.ValidateObjectFunc, _ bool) (runtime.Object, error)
}

type storage struct {
	Storage
}

func NewRegistry(s Storage) Registry {
	return &storage{s}
}

func (s *storage) CreateSubjectAccessReview(ctx context.Context, subjectAccessReview *api.SubjectAccessReview) (*api.SubjectAccessReviewResponse, error) {
	obj, err := s.Create(ctx, subjectAccessReview, nil, false)
	if err != nil {
		return nil, err
	}
	return obj.(*api.SubjectAccessReviewResponse), nil
}
