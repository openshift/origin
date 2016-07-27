package cluster

import (
	"fmt"
	"io/ioutil"

	kerrs "k8s.io/kubernetes/pkg/api/errors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	osclient "github.com/openshift/origin/pkg/client"
	policycmd "github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// ClusterRoles is a Diagnostic to check that the default cluster roles match expectations
type ClusterRoles struct {
	ClusterRolesClient osclient.ClusterRolesInterface
	SARClient          osclient.SubjectAccessReviews
}

const (
	ClusterRolesName = "ClusterRoles"
)

func (d *ClusterRoles) Name() string {
	return ClusterRolesName
}

func (d *ClusterRoles) Description() string {
	return "Check that the default ClusterRoles are present and contain the expected permissions"
}

func (d *ClusterRoles) CanRun() (bool, error) {
	if d.ClusterRolesClient == nil {
		return false, fmt.Errorf("must have client.ClusterRolesInterface")
	}
	if d.SARClient == nil {
		return false, fmt.Errorf("must have client.SubjectAccessReviews")
	}

	return userCan(d.SARClient, authorizationapi.Action{
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
		RoleClient: d.ClusterRolesClient.ClusterRoles(),
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
		actualClusterRole, err := d.ClusterRolesClient.ClusterRoles().Get(changedClusterRole.Name)
		if kerrs.IsNotFound(err) {
			r.Error("CRD1002", nil, fmt.Sprintf("clusterrole/%s is missing.\n\nUse the `oadm policy reconcile-cluster-roles` command to create the role.", changedClusterRole.Name))
			continue
		}
		if err != nil {
			r.Error("CRD1001", err, fmt.Sprintf("Unable to get clusterrole/%s: %v", changedClusterRole.Name, err))
		}

		_, missingRules := rulevalidation.Covers(actualClusterRole.Rules, changedClusterRole.Rules)
		if len(missingRules) == 0 {
			r.Warn("CRD1003", nil, fmt.Sprintf("clusterrole/%s has changed, but the existing role has more permissions than the new role.\n\nUse the `oadm policy reconcile-cluster-roles` command to update the role to reduce permissions.", changedClusterRole.Name))
			_, extraRules := rulevalidation.Covers(changedClusterRole.Rules, actualClusterRole.Rules)
			for _, extraRule := range extraRules {
				r.Info("CRD1008", fmt.Sprintf("clusterrole/%s has extra permission %v.", changedClusterRole.Name, extraRule))
			}
			continue
		}

		r.Error("CRD1005", nil, fmt.Sprintf("clusterrole/%s has changed and the existing role does not have enough permissions.\n\nUse the `oadm policy reconcile-cluster-roles` command to update the role.", changedClusterRole.Name))
		for _, missingRule := range missingRules {
			r.Info("CRD1007", fmt.Sprintf("clusterrole/%s is missing permission %v.", changedClusterRole.Name, missingRule))
		}
		r.Debug("CRD1006", fmt.Sprintf("clusterrole/%s is now %v.", changedClusterRole.Name, changedClusterRole))
	}

	return r
}
