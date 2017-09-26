package cluster

import (
	"fmt"
	"io/ioutil"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/authorization"
	authorizationtypedclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	oauthorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	regutil "github.com/openshift/origin/pkg/authorization/registry/util"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/util"
	policycmd "github.com/openshift/origin/pkg/oc/admin/policy"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"
)

// ClusterRoles is a Diagnostic to check that the default cluster roles match expectations
type ClusterRoles struct {
	ClusterRolesClient oauthorizationtypedclient.ClusterRoleInterface
	SARClient          authorizationtypedclient.SelfSubjectAccessReviewsGetter
}

const (
	ClusterRolesName   = "ClusterRoles"
	clusterRoleMissing = `
clusterrole/%s is missing.

Use the 'oc adm policy reconcile-cluster-roles' command to create the role. For example,

  $ oc adm policy reconcile-cluster-roles \
         --additive-only=true --confirm
`
	clusterRoleReduced = `
clusterrole/%s has changed, but the existing role has more permissions than the new role.

If you can confirm that the extra permissions are not required, you may use the
'oc adm policy reconcile-cluster-roles' command to update the role to reduce permissions.
For example,

  $ oc adm policy reconcile-cluster-roles \
         --additive-only=false --confirm
`
	clusterRoleChanged = `
clusterrole/%s has changed and the existing role does not have enough permissions.

Use the 'oc adm policy reconcile-cluster-roles' command to update the role.
For example,

  $ oc adm policy reconcile-cluster-roles \
         --additive-only=true --confirm
`
)

func (d *ClusterRoles) Name() string {
	return ClusterRolesName
}

func (d *ClusterRoles) Description() string {
	return "Check that the default ClusterRoles are present and contain the expected permissions"
}

func (d *ClusterRoles) Requirements() (client bool, host bool) {
	return true, false
}

func (d *ClusterRoles) CanRun() (bool, error) {
	if d.ClusterRolesClient == nil {
		return false, fmt.Errorf("must have client.ClusterRolesInterface")
	}
	if d.SARClient == nil {
		return false, fmt.Errorf("must have client.SubjectAccessReviews")
	}

	return util.UserCan(d.SARClient, &authorization.ResourceAttributes{
		Verb:     "list",
		Group:    authorizationapi.GroupName,
		Resource: "clusterroles",
	})
}

func (d *ClusterRoles) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(ClusterRolesName)

	reconcileOptions := &policycmd.ReconcileClusterRolesOptions{
		Confirmed:  false,
		Union:      false,
		Out:        ioutil.Discard,
		RoleClient: d.ClusterRolesClient,
	}

	changedClusterRoles, _, err := reconcileOptions.ChangedClusterRoles()
	if err != nil {
		r.Error("CRD1000", err, fmt.Sprintf("Error inspecting ClusterRoles: %v", err))
		return r
	}

	// success
	if len(changedClusterRoles) == 0 {
		return r
	}

	for _, changedClusterRole := range changedClusterRoles {
		actualClusterRole, err := d.ClusterRolesClient.Get(changedClusterRole.Name, metav1.GetOptions{})
		if kerrs.IsNotFound(err) {
			r.Error("CRD1002", nil, fmt.Sprintf(clusterRoleMissing, changedClusterRole.Name))
			continue
		}
		if err != nil {
			r.Error("CRD1001", err, fmt.Sprintf("Unable to get clusterrole/%s: %v", changedClusterRole.Name, err))
			continue
		}

		actualRBACClusterRole, err := regutil.ClusterRoleToRBAC(actualClusterRole)
		if err != nil {
			r.Error("CRD1009", err, fmt.Sprintf("Unable to convert clusterrole/%s to RBAC cluster role: %v", actualClusterRole.Name, err))
			continue
		}

		_, missingRules := rbacregistryvalidation.Covers(actualRBACClusterRole.Rules, changedClusterRole.Rules)
		if len(missingRules) == 0 {
			r.Info("CRD1003", fmt.Sprintf(clusterRoleReduced, changedClusterRole.Name))
			_, extraRules := rbacregistryvalidation.Covers(changedClusterRole.Rules, actualRBACClusterRole.Rules)
			for _, extraRule := range extraRules {
				r.Info("CRD1008", fmt.Sprintf("clusterrole/%s has extra permission %v.", changedClusterRole.Name, extraRule))
			}
			continue
		}

		r.Error("CRD1005", nil, fmt.Sprintf(clusterRoleChanged, changedClusterRole.Name))
		for _, missingRule := range missingRules {
			r.Info("CRD1007", fmt.Sprintf("clusterrole/%s is missing permission %v.", changedClusterRole.Name, missingRule))
		}
		r.Debug("CRD1006", fmt.Sprintf("clusterrole/%s is now %v.", changedClusterRole.Name, changedClusterRole))
	}

	return r
}
