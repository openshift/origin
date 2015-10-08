package cluster

import (
	"fmt"
	"io/ioutil"

	kerrs "k8s.io/kubernetes/pkg/api/errors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	osclient "github.com/openshift/origin/pkg/client"
	policycmd "github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// ClusterRoleBindings is a Diagnostic to check that the default cluster role bindings match expectations
type ClusterRoleBindings struct {
	ClusterRoleBindingsClient osclient.ClusterRoleBindingsInterface
	SARClient                 osclient.SubjectAccessReviews
}

const (
	ClusterRoleBindingsName = "ClusterRoleBindings"
)

func (d *ClusterRoleBindings) Name() string {
	return ClusterRoleBindingsName
}

func (d *ClusterRoleBindings) Description() string {
	return "Check that the ClusterRoleBindings are up-to-date"
}

func (d *ClusterRoleBindings) CanRun() (bool, error) {
	if d.ClusterRoleBindingsClient == nil {
		return false, fmt.Errorf("must have client.ClusterRoleBindingsInterface")
	}
	if d.SARClient == nil {
		return false, fmt.Errorf("must have client.SubjectAccessReviews")
	}

	return userCan(d.SARClient, authorizationapi.AuthorizationAttributes{
		Verb:     "list",
		Resource: "clusterrolebindings",
	})
}

func (d *ClusterRoleBindings) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(ClusterRoleBindingsName)

	reconcileOptions := &policycmd.ReconcileClusterRoleBindingsOptions{
		Confirmed:         false,
		Union:             false,
		Out:               ioutil.Discard,
		RoleBindingClient: d.ClusterRoleBindingsClient.ClusterRoleBindings(),
	}

	changedClusterRoleBindings, err := reconcileOptions.ChangedClusterRoleBindings()
	if err != nil {
		r.Error("CRBD1000", err, fmt.Sprintf("Error inspecting ClusterRoleBindings: %v", err))
	}

	// success
	if len(changedClusterRoleBindings) == 0 {
		return r
	}

	for _, changedClusterRoleBinding := range changedClusterRoleBindings {
		actualClusterRole, err := d.ClusterRoleBindingsClient.ClusterRoleBindings().Get(changedClusterRoleBinding.Name)
		if kerrs.IsNotFound(err) {
			r.Error("CRBD1001", nil, fmt.Sprintf("clusterrolebinding/%s is missing.\n\nUse the `oadm policy reconcile-cluster-role-bindings` command to create the role binding.", changedClusterRoleBinding.Name))
			continue
		}
		if err != nil {
			r.Error("CRBD1002", err, fmt.Sprintf("Unable to get clusterrolebinding/%s: %v", changedClusterRoleBinding.Name, err))
		}

		missingSubjects, extraSubjects := policycmd.Diff(changedClusterRoleBinding.Subjects, actualClusterRole.Subjects)
		switch {
		case len(missingSubjects) > 0:
			// Only a warning, because they can remove things like self-provisioner role from system:unauthenticated, and it's not an error
			r.Warn("CRBD1003", nil, fmt.Sprintf("clusterrolebinding/%s is missing expected subjects.\n\nUse the `oadm policy reconcile-cluster-role bindings` command to update the role binding to include expected subjects.", changedClusterRoleBinding.Name))
		case len(extraSubjects) > 0:
			// Only info, because it is normal to use policy to grant cluster roles to users
			r.Info("CRBD1004", fmt.Sprintf("clusterrolebinding/%s has more subjects than expected.\n\nUse the `oadm policy reconcile-cluster-role bindings` command to update the role binding to remove extra subjects.", changedClusterRoleBinding.Name))
		}

		for _, missingSubject := range missingSubjects {
			r.Info("CRBD1005", fmt.Sprintf("clusterrolebinding/%s is missing subject %v.", changedClusterRoleBinding.Name, missingSubject))
		}

		for _, extraSubject := range extraSubjects {
			r.Info("CRBD1006", fmt.Sprintf("clusterrolebinding/%s has extra subject %v.", changedClusterRoleBinding.Name, extraSubject))
		}

		r.Debug("CRBD1007", fmt.Sprintf("clusterrolebinding/%s is now %v.", changedClusterRoleBinding.Name, changedClusterRoleBinding))
	}

	return r
}
