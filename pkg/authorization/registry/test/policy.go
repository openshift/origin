package test

import (
	"errors"
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type PolicyRegistry struct {
	Err               error
	MasterNamespace   string
	Policies          []authorizationapi.Policy
	DeletedPolicyName string
}

// ListPolicies obtains list of policies that match a selector.
func (r *PolicyRegistry) ListPolicies(ctx kapi.Context, labels, fields klabels.Selector) (*authorizationapi.PolicyList, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	namespace := kapi.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return nil, errors.New("invalid request.  Namespace parameter required.")
	}

	list := make([]authorizationapi.Policy, 0)
	for _, curr := range r.Policies {
		if curr.Namespace == namespace {
			list = append(list, curr)
		}
	}

	return &authorizationapi.PolicyList{
			Items: list,
		},
		r.Err
}

// GetPolicy retrieves a specific policy.
func (r *PolicyRegistry) GetPolicy(ctx kapi.Context, id string) (*authorizationapi.Policy, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	namespace := kapi.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return nil, errors.New("invalid request.  Namespace parameter required.")
	}

	for _, curr := range r.Policies {
		if curr.Namespace == namespace && id == curr.Name {
			return &curr, nil
		}
	}

	return nil, fmt.Errorf("Policy %v::%v not found", namespace, id)
}

// CreatePolicy creates a new policy.
func (r *PolicyRegistry) CreatePolicy(ctx kapi.Context, policy *authorizationapi.Policy) error {
	return r.Err
}

// UpdatePolicy updates a policy.
func (r *PolicyRegistry) UpdatePolicy(ctx kapi.Context, policy *authorizationapi.Policy) error {
	return r.Err
}

// DeletePolicy deletes a policy.
func (r *PolicyRegistry) DeletePolicy(ctx kapi.Context, id string) error {
	r.DeletedPolicyName = id
	return r.Err
}

func (r *PolicyRegistry) WatchPolicies(ctx kapi.Context, label, field klabels.Selector, resourceVersion string) (watch.Interface, error) {
	return nil, r.Err
}
