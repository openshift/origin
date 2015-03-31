package policybinding

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/api/validation"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewREST creates a new REST for policyBindings.
func NewREST(registry Registry) *REST {
	return &REST{registry}
}

// New creates a new PolicyBinding object
func (r *REST) New() runtime.Object {
	return &authorizationapi.PolicyBinding{}
}

func (r *REST) NewList() runtime.Object {
	return &authorizationapi.PolicyBindingList{}
}

// List obtains a list of PolicyBindings that match selector.
func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	policyBindings, err := r.registry.ListPolicyBindings(ctx, label, field)
	if err != nil {
		return nil, err
	}
	return policyBindings, err

}

// Get obtains the policyBinding specified by its id.
func (r *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	policyBinding, err := r.registry.GetPolicyBinding(ctx, id)
	if err != nil {
		return nil, err
	}
	return policyBinding, err
}

// Delete asynchronously deletes the PolicyBinding specified by its id.
func (r *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	return &kapi.Status{Status: kapi.StatusSuccess}, r.registry.DeletePolicyBinding(ctx, id)
}

// Create registers a given new PolicyBinding instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	policyBinding, ok := obj.(*authorizationapi.PolicyBinding)
	if !ok {
		return nil, fmt.Errorf("not an policyBinding: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &policyBinding.ObjectMeta) {
		return nil, kerrors.NewConflict("policyBinding", policyBinding.Namespace, fmt.Errorf("PolicyBinding.Namespace does not match the provided context"))
	}

	kapi.FillObjectMetaSystemFields(ctx, &policyBinding.ObjectMeta)
	policyBinding.Name = policyBinding.PolicyRef.Namespace
	policyBinding.LastModified = util.Now()
	if errs := validation.ValidatePolicyBinding(policyBinding); len(errs) > 0 {
		return nil, kerrors.NewInvalid("policyBinding", policyBinding.Name, errs)
	}

	if err := r.registry.CreatePolicyBinding(ctx, policyBinding); err != nil {
		return nil, err
	}
	return r.Get(ctx, policyBinding.Name)
}

// Watch begins watching for new, changed, or deleted PolicyBindings.
func (r *REST) Watch(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return r.registry.WatchPolicyBindings(ctx, label, field, resourceVersion)
}

func NewEmptyPolicyBinding(namespace, policyNamespace string) *authorizationapi.PolicyBinding {
	policyBinding := &authorizationapi.PolicyBinding{}
	policyBinding.Name = policyNamespace
	policyBinding.Namespace = namespace
	policyBinding.CreationTimestamp = util.Now()
	policyBinding.LastModified = util.Now()
	policyBinding.PolicyRef = kapi.ObjectReference{Name: authorizationapi.PolicyName, Namespace: policyNamespace}
	policyBinding.RoleBindings = make(map[string]authorizationapi.RoleBinding)

	return policyBinding
}
