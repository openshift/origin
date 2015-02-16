package rolebinding

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Registry is an interface for things that know how to store RoleBindings.
type Registry interface {
	// ListRoleBindings obtains list of policyRoleBindings that match a selector.
	ListRoleBindings(ctx kapi.Context, labels, fields klabels.Selector) (*authorizationapi.RoleBindingList, error)
	// GetRoleBinding retrieves a specific policyRoleBinding.
	GetRoleBinding(ctx kapi.Context, id string) (*authorizationapi.RoleBinding, error)
	// CreateRoleBinding creates a new policyRoleBinding.
	CreateRoleBinding(ctx kapi.Context, policyRoleBinding *authorizationapi.RoleBinding) error
	// UpdateRoleBinding updates a policyRoleBinding.
	UpdateRoleBinding(ctx kapi.Context, policyRoleBinding *authorizationapi.RoleBinding) error
	// DeleteRoleBinding deletes a policyRoleBinding.
	DeleteRoleBinding(ctx kapi.Context, id string) error
}
