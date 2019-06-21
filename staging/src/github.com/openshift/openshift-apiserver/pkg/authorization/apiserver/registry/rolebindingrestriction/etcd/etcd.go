package etcd

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/watch"

	"k8s.io/apimachinery/pkg/api/errors"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	authorizationapi "github.com/openshift/openshift-apiserver/pkg/authorization/apis/authorization"
)

type REST struct {
}

var _ rest.StandardStorage = &REST{}
var _ rest.Scoper = &REST{}

// NewREST returns a RESTStorage object that will work against nodes.
func NewREST() (*REST, error) {
	return &REST{}, nil
}

func (r *REST) NamespaceScoped() bool {
	return true
}

func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return nil, errors.NewInternalError(fmt.Errorf("unsupported"))
}

func (r *REST) NewList() runtime.Object {
	return &authorizationapi.RoleBindingRestrictionList{}
}

func (r *REST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	return nil, errors.NewInternalError(fmt.Errorf("unsupported"))
}

func (r *REST) New() runtime.Object {
	return &authorizationapi.RoleBindingRestriction{}
}

func (r *REST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	return nil, errors.NewInternalError(fmt.Errorf("unsupported"))
}

func (r *REST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return nil, false, errors.NewInternalError(fmt.Errorf("unsupported"))
}

func (r *REST) Delete(ctx context.Context, name string, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	return nil, false, errors.NewInternalError(fmt.Errorf("unsupported"))
}

func (r *REST) DeleteCollection(ctx context.Context, options *metav1.DeleteOptions, listOptions *metainternalversion.ListOptions) (runtime.Object, error) {
	return nil, errors.NewInternalError(fmt.Errorf("unsupported"))
}

func (r *REST) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return nil, errors.NewInternalError(fmt.Errorf("unsupported"))
}
