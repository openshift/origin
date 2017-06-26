package v1_test

import (
	"testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	_ "github.com/openshift/origin/pkg/authorization/apis/authorization/install"
	testutil "github.com/openshift/origin/test/util/api"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "ClusterPolicy",
		// Ensure all currently returned labels are supported
		authorizationapi.ClusterPolicyToSelectableFields(&authorizationapi.ClusterPolicy{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "ClusterPolicyBinding",
		// Ensure all currently returned labels are supported
		authorizationapi.ClusterPolicyBindingToSelectableFields(&authorizationapi.ClusterPolicyBinding{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "Policy",
		// Ensure all currently returned labels are supported
		authorizationapi.PolicyToSelectableFields(&authorizationapi.Policy{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "PolicyBinding",
		// Ensure all currently returned labels are supported
		authorizationapi.PolicyBindingToSelectableFields(&authorizationapi.PolicyBinding{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "Role",
		// Ensure all currently returned labels are supported
		authorizationapi.RoleToSelectableFields(&authorizationapi.Role{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "RoleBinding",
		// Ensure all currently returned labels are supported
		authorizationapi.RoleBindingToSelectableFields(&authorizationapi.RoleBinding{}),
	)

}
