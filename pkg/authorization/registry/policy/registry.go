package policy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Registry is an interface for things that know how to store Policies.
type Registry interface {
	// ListPolicies obtains list of policies that match a selector.
	ListPolicies(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.PolicyList, error)
	// GetPolicy retrieves a specific policy.
	GetPolicy(ctx kapi.Context, id string) (*authorizationapi.Policy, error)
	// CreatePolicy creates a new policy.
	CreatePolicy(ctx kapi.Context, policy *authorizationapi.Policy) error
	// UpdatePolicy updates a policy.
	UpdatePolicy(ctx kapi.Context, policy *authorizationapi.Policy) error
	// DeletePolicy deletes a policy.
	DeletePolicy(ctx kapi.Context, id string) error
	// WatchPolicyBindings watches policyBindings.
	WatchPolicies(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
}
