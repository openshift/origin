package policybinding

import (
	"errors"
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewREST creates a new REST for policyBindings.
func NewREST(registry Registry) apiserver.RESTStorage {
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
func (r *REST) List(ctx kapi.Context, selector, fields klabels.Selector) (runtime.Object, error) {
	policyBindings, err := r.registry.ListPolicyBindings(ctx, selector, fields)
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
func (r *REST) Delete(ctx kapi.Context, id string) (<-chan apiserver.RESTResult, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return &kapi.Status{Status: kapi.StatusSuccess}, r.registry.DeletePolicyBinding(ctx, id)
	}), nil
}

// Create registers a given new PolicyBinding instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	policyBinding, ok := obj.(*authorizationapi.PolicyBinding)
	if !ok {
		return nil, fmt.Errorf("not an policyBinding: %#v", obj)
	}

	if !kapi.ValidNamespace(ctx, &policyBinding.ObjectMeta) {
		return nil, kerrors.NewConflict("policyBinding", policyBinding.Namespace, fmt.Errorf("PolicyBinding.Namespace does not match the provided context"))
	}

	if len(policyBinding.PolicyRef.Namespace) == 0 {
		return nil, errors.New("policyBinding.PolicyRef.Namespace must have a value")
	}

	// set values
	policyBinding.Name = policyBinding.PolicyRef.Namespace
	policyBinding.CreationTimestamp = util.Now()
	policyBinding.LastModified = util.Now()

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		if err := r.registry.CreatePolicyBinding(ctx, policyBinding); err != nil {
			return nil, err
		}
		return r.Get(ctx, policyBinding.Name)
	}), nil
}

func EmptyPolicyBinding(namespace, policyNamespace string) *authorizationapi.PolicyBinding {
	policyBinding := &authorizationapi.PolicyBinding{}
	policyBinding.Name = policyNamespace
	policyBinding.Namespace = namespace
	policyBinding.CreationTimestamp = util.Now()
	policyBinding.LastModified = util.Now()
	policyBinding.PolicyRef = kapi.ObjectReference{Name: authorizationapi.PolicyName, Namespace: policyNamespace}
	policyBinding.RoleBindings = make(map[string]authorizationapi.RoleBinding)

	return policyBinding
}
