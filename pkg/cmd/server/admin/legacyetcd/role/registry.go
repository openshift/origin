package role

import (
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// Storage is an interface for a standard REST Storage backend
type Storage interface {
	rest.Getter
	rest.Lister
	rest.CreaterUpdater
	rest.GracefulDeleter

	// CreateRoleWithEscalation creates a new policyRole.  Skipping the escalation check should only be done during bootstrapping procedures where no users are currently bound.
	CreateRoleWithEscalation(ctx apirequest.Context, policyRole *authorizationapi.Role) (*authorizationapi.Role, error)
	// UpdateRoleWithEscalation updates a policyRole.  Skipping the escalation check should only be done during bootstrapping procedures where no users are currently bound.
	UpdateRoleWithEscalation(ctx apirequest.Context, policyRole *authorizationapi.Role) (*authorizationapi.Role, bool, error)
}
