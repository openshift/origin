package impersonatingclient

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/endpoints/request"
	rbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/rest"
)

func NewImpersonatingRBACFromContext(ctx context.Context, restclient rest.Interface) (rbacv1.RbacV1Interface, error) {
	user, ok := request.UserFrom(ctx)
	if !ok {
		return nil, apierrors.NewBadRequest("user missing from context")
	}
	return rbacv1.New(NewImpersonatingRESTClient(user, restclient)), nil
}
