package resourceaccessreview

import (
	api "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
)

type Registry interface {
	CreateResourceAccessReview(ctx apirequest.Context, resourceAccessReview *api.ResourceAccessReview) (*api.ResourceAccessReviewResponse, error)
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

func (s *storage) CreateResourceAccessReview(ctx apirequest.Context, resourceAccessReview *api.ResourceAccessReview) (*api.ResourceAccessReviewResponse, error) {
	obj, err := s.Create(ctx, resourceAccessReview, false)
	if err != nil {
		return nil, err
	}
	return obj.(*api.ResourceAccessReviewResponse), nil
}
