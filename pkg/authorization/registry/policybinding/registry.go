package policybinding

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Registry is an interface for things that know how to store Policies.
type Registry interface {
	// ListPolicyBindings obtains list of policyBindings that match a selector.
	ListPolicyBindings(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.PolicyBindingList, error)
	// GetPolicyBinding retrieves a specific policyBinding.
	GetPolicyBinding(ctx kapi.Context, id string) (*authorizationapi.PolicyBinding, error)
	// CreatePolicyBinding creates a new policyBinding.
	CreatePolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.PolicyBinding) error
	// UpdatePolicyBinding updates a policyBinding.
	UpdatePolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.PolicyBinding) error
	// DeletePolicyBinding deletes a policyBinding.
	DeletePolicyBinding(ctx kapi.Context, id string) error
	// WatchPolicyBindings watches policyBindings.
	WatchPolicyBindings(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
}
