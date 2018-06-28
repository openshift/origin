package resourceaccessreview

import (
	"context"

	api "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

type Registry interface {
	CreateResourceAccessReview(ctx context.Context, resourceAccessReview *api.ResourceAccessReview) (*api.ResourceAccessReviewResponse, error)
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

func (s *storage) CreateResourceAccessReview(ctx context.Context, resourceAccessReview *api.ResourceAccessReview) (*api.ResourceAccessReviewResponse, error) {
	obj, err := s.Create(ctx, resourceAccessReview, nil, false)
	if err != nil {
		return nil, err
	}
	return obj.(*api.ResourceAccessReviewResponse), nil
}
