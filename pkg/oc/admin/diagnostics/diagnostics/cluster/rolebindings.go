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
)

// ClusterRoleBindings is a Diagnostic to check that the default cluster role bindings match expectations
type ClusterRoleBindings struct {
	ClusterRoleBindingsClient oauthorizationtypedclient.ClusterRoleBindingInterface
	SARClient                 authorizationtypedclient.SelfSubjectAccessReviewsGetter
}

const (
	ClusterRoleBindingsName = "ClusterRoleBindings"
)

func (d *ClusterRoleBindings) Name() string {
	return ClusterRoleBindingsName
}

func (d *ClusterRoleBindings) Description() string {
	return "Check that the default ClusterRoleBindings are present and contain the expected subjects"
}

func (d *ClusterRoleBindings) Requirements() (client bool, host bool) {
	return true, false
}

func (d *ClusterRoleBindings) CanRun() (bool, error) {
	if d.ClusterRoleBindingsClient == nil {
		return false, fmt.Errorf("must have client.ClusterRoleBindingsInterface")
	}
	if d.SARClient == nil {
		return false, fmt.Errorf("must have client.SubjectAccessReviews")
	}

	return util.UserCan(d.SARClient, &authorization.ResourceAttributes{
		Verb:     "list",
		Group:    authorizationapi.GroupName,
		Resource: "clusterrolebindings",
	})
}

func (d *ClusterRoleBindings) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(ClusterRoleBindingsName)

	reconcileOptions := &policycmd.ReconcileClusterRoleBindingsOptions{
		Confirmed:         false,
		Union:             false,
		Out:               ioutil.Discard,
		RoleBindingClient: d.ClusterRoleBindingsClient,
	}

	changedClusterRoleBindings, _, err := reconcileOptions.ChangedClusterRoleBindings()
	if policycmd.IsClusterRoleBindingLookupError(err) {
		// we got a partial match, so we log the error that stopped us from getting a full match
		// but continue to interpret the partial results that we did get
		r.Warn("CRBD1008", err, fmt.Sprintf("Error finding ClusterRoleBindings: %v", err))
	} else if err != nil {
		r.Error("CRBD1000", err, fmt.Sprintf("Error inspecting ClusterRoleBindings: %v", err))
		return r
	}

	// success
	if len(changedClusterRoleBindings) == 0 {
		return r
	}

	for _, changedClusterRoleBinding := range changedClusterRoleBindings {
		actualClusterRole, err := d.ClusterRoleBindingsClient.Get(changedClusterRoleBinding.Name, metav1.GetOptions{})
		if kerrs.IsNotFound(err) {
			r.Error("CRBD1001", nil, fmt.Sprintf("clusterrolebinding/%s is missing.\n\nUse the `oc adm policy reconcile-cluster-role-bindings` command to create the role binding.", changedClusterRoleBinding.Name))
			continue
		}
		if err != nil {
			r.Error("CRBD1002", err, fmt.Sprintf("Unable to get clusterrolebinding/%s: %v", changedClusterRoleBinding.Name, err))
			continue
		}
		actualRBACClusterRole, err := regutil.ClusterRoleBindingToRBAC(actualClusterRole)
		if err != nil {
			r.Error("CRBD1008", err, fmt.Sprintf("Unable to convert clusterrolebinding/%s to RBAC: %v", actualClusterRole.Name, err))
			continue
		}

		missingSubjects, extraSubjects := policycmd.DiffSubjects(changedClusterRoleBinding.Subjects, actualRBACClusterRole.Subjects)
		switch {
		case len(missingSubjects) > 0:
			// Only a warning, because they can remove things like self-provisioner role from system:unauthenticated, and it's not an error
			r.Warn("CRBD1003", nil, fmt.Sprintf("clusterrolebinding/%s is missing expected subjects.\n\nUse the `oc adm policy reconcile-cluster-role-bindings` command to update the role binding to include expected subjects.", changedClusterRoleBinding.Name))
		case len(extraSubjects) > 0:
			// Only info, because it is normal to use policy to grant cluster roles to users
			r.Info("CRBD1004", fmt.Sprintf("clusterrolebinding/%s has more subjects than expected.\n\nUse the `oc adm policy reconcile-cluster-role-bindings` command to update the role binding to remove extra subjects.", changedClusterRoleBinding.Name))
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
