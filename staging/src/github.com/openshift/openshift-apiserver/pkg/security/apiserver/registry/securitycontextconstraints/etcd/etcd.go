package etcd

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/registry/rest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	securityapi "github.com/openshift/openshift-apiserver/pkg/security/apis/security"
)

// REST implements a RESTStorage for security context constraints against etcd
type REST struct {
}

var _ rest.StandardStorage = &REST{}
var _ rest.ShortNamesProvider = &REST{}
var _ rest.Scoper = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"scc"}
}

// NewREST returns a RESTStorage object that will work against security context constraints objects.
func NewREST() *REST {
	return &REST{}
}

// LegacyREST allows us to wrap and alter some behavior
type LegacyREST struct {
	*REST
}

func (r *LegacyREST) Categories() []string {
	return []string{}
}

func (r *REST) NamespaceScoped() bool {
	return false
}

func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return nil, errors.NewInternalError(fmt.Errorf("unsupported"))
}

func (r *REST) NewList() runtime.Object {
	return &securityapi.SecurityContextConstraintsList{}
}

func (r *REST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	return nil, errors.NewInternalError(fmt.Errorf("unsupported"))
}

func (r *REST) New() runtime.Object {
	return &securityapi.SecurityContextConstraints{}
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
