package role

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Registry is an interface for things that know how to store Roles.
type Registry interface {
	// ListRoles obtains list of policyRoles that match a selector.
	ListRoles(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.RoleList, error)
	// GetRole retrieves a specific policyRole.
	GetRole(ctx kapi.Context, id string) (*authorizationapi.Role, error)
	// CreateRole creates a new policyRole.
	CreateRole(ctx kapi.Context, policyRole *authorizationapi.Role) error
	// UpdateRole updates a policyRole.
	UpdateRole(ctx kapi.Context, policyRole *authorizationapi.Role) error
	// DeleteRole deletes a policyRole.
	DeleteRole(ctx kapi.Context, id string) error
}
