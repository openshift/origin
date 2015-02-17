package test

import (
	"errors"
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type PolicyBindingRegistry struct {
	Err                      error
	MasterNamespace          string
	PolicyBindings           []authorizationapi.PolicyBinding
	DeletedPolicyBindingName string
}

// ListPolicies obtains list of ListPolicyBinding that match a selector.
func (r *PolicyBindingRegistry) ListPolicyBindings(ctx kapi.Context, labels, fields klabels.Selector) (*authorizationapi.PolicyBindingList, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	namespace := kapi.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return nil, errors.New("invalid request.  Namespace parameter required.")
	}

	list := make([]authorizationapi.PolicyBinding, 0)
	for _, curr := range r.PolicyBindings {
		if curr.Namespace == namespace {
			list = append(list, curr)
		}
	}

	return &authorizationapi.PolicyBindingList{
			Items: list,
		},
		r.Err
}

// GetPolicyBinding retrieves a specific policyBinding.
func (r *PolicyBindingRegistry) GetPolicyBinding(ctx kapi.Context, id string) (*authorizationapi.PolicyBinding, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	namespace := kapi.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return nil, errors.New("invalid request.  Namespace parameter required.")
	}

	for _, curr := range r.PolicyBindings {
		if curr.Namespace == namespace && id == curr.Name {
			return &curr, nil
		}
	}

	return nil, fmt.Errorf("PolicyBinding %v::%v not found", namespace, id)
}

// CreatePolicyBinding creates a new policyBinding.
func (r *PolicyBindingRegistry) CreatePolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.PolicyBinding) error {
	if r.Err == nil {
		r.PolicyBindings = append(r.PolicyBindings, *policyBinding)
	}
	return r.Err
}

// UpdatePolicyBinding updates a policyBinding.
func (r *PolicyBindingRegistry) UpdatePolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.PolicyBinding) error {
	return r.Err
}

// DeletePolicyBinding deletes a policyBinding.
func (r *PolicyBindingRegistry) DeletePolicyBinding(ctx kapi.Context, id string) error {
	r.DeletedPolicyBindingName = id
	return r.Err
}

func (r *PolicyBindingRegistry) WatchPolicyBindings(ctx kapi.Context, label, field klabels.Selector, resourceVersion string) (watch.Interface, error) {
	return nil, r.Err
}
