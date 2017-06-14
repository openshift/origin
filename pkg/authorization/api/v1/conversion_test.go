package v1_test

import (
	"testing"

	"github.com/openshift/origin/pkg/authorization/api"
	_ "github.com/openshift/origin/pkg/authorization/api/install"
	"github.com/openshift/origin/pkg/authorization/api/v1"
	testutil "github.com/openshift/origin/test/util/api"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "ClusterPolicy",
		// Ensure all currently returned labels are supported
		api.ClusterPolicyToSelectableFields(&api.ClusterPolicy{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "ClusterPolicyBinding",
		// Ensure all currently returned labels are supported
		api.ClusterPolicyBindingToSelectableFields(&api.ClusterPolicyBinding{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "Policy",
		// Ensure all currently returned labels are supported
		api.PolicyToSelectableFields(&api.Policy{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "PolicyBinding",
		// Ensure all currently returned labels are supported
		api.PolicyBindingToSelectableFields(&api.PolicyBinding{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "Role",
		// Ensure all currently returned labels are supported
		api.RoleToSelectableFields(&api.Role{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "RoleBinding",
		// Ensure all currently returned labels are supported
		api.RoleBindingToSelectableFields(&api.RoleBinding{}),
	)

}

func TestEmptySlice(t *testing.T) {
	{
		in := &v1.SubjectAccessReview{}
		out := &api.SubjectAccessReview{}
		if err := v1.Convert_v1_SubjectAccessReview_To_api_SubjectAccessReview(in, out, nil); err != nil || out.Scopes != nil {
			t.Errorf("expected no error, nil scopes, got %v, %#v", err, out.Scopes)
		}
	}

	{
		in := &v1.SubjectAccessReview{Scopes: v1.OptionalScopes{}}
		out := &api.SubjectAccessReview{}
		if err := v1.Convert_v1_SubjectAccessReview_To_api_SubjectAccessReview(in, out, nil); err != nil || out.Scopes == nil {
			t.Errorf("expected no error, non-nil scopes, got %v, %#v", err, out.Scopes)
		}
	}
}
