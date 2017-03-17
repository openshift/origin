package v1_test

import (
	"testing"

	"github.com/openshift/origin/pkg/authorization/api"
	_ "github.com/openshift/origin/pkg/authorization/api/install"
	testutil "github.com/openshift/origin/test/util/api"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "ClusterPolicy", false,
		// Ensure all currently returned labels are supported
		api.ClusterPolicyToSelectableFields(&api.ClusterPolicy{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "ClusterPolicyBinding", false,
		// Ensure all currently returned labels are supported
		api.ClusterPolicyBindingToSelectableFields(&api.ClusterPolicyBinding{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "Policy", true,
		// Ensure all currently returned labels are supported
		api.PolicyToSelectableFields(&api.Policy{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "PolicyBinding", true,
		// Ensure all currently returned labels are supported
		api.PolicyBindingToSelectableFields(&api.PolicyBinding{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "Role", true,
		// Ensure all currently returned labels are supported
		api.RoleToSelectableFields(&api.Role{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "RoleBinding", true,
		// Ensure all currently returned labels are supported
		api.RoleBindingToSelectableFields(&api.RoleBinding{}),
	)

}
