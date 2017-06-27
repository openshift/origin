package clusterrolebinding

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

	// CreateRoleBinding creates a new policyRoleBinding.  Skipping the escalation check should only be done during bootstrapping procedures where no users are currently bound.
	CreateClusterRoleBindingWithEscalation(ctx apirequest.Context, policyRoleBinding *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error)
	// UpdateRoleBinding updates a policyRoleBinding.  Skipping the escalation check should only be done during bootstrapping procedures where no users are currently bound.
	UpdateClusterRoleBindingWithEscalation(ctx apirequest.Context, policyRoleBinding *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, bool, error)
}
