package rolebinding

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Registry is an interface for things that know how to store RoleBindings.
type Registry interface {
	// ListRoleBindings obtains list of policyRoleBindings that match a selector.
	ListRoleBindings(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.RoleBindingList, error)
	// GetRoleBinding retrieves a specific policyRoleBinding.
	GetRoleBinding(ctx kapi.Context, id string) (*authorizationapi.RoleBinding, error)
	// CreateRoleBinding creates a new policyRoleBinding.  Skipping the escalation check should only be done during bootstrapping procedures where no users are currently bound.
	CreateRoleBinding(ctx kapi.Context, policyRoleBinding *authorizationapi.RoleBinding, allowEscalation bool) error
	// UpdateRoleBinding updates a policyRoleBinding.  Skipping the escalation check should only be done during bootstrapping procedures where no users are currently bound.
	UpdateRoleBinding(ctx kapi.Context, policyRoleBinding *authorizationapi.RoleBinding, allowEscalation bool) error
	// DeleteRoleBinding deletes a policyRoleBinding.
	DeleteRoleBinding(ctx kapi.Context, id string) error
}
