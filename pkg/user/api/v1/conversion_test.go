package v1

import (
	"testing"

	"github.com/openshift/origin/pkg/user/api"
	testutil "github.com/openshift/origin/test/util/api"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "Group",
		// Ensure all currently returned labels are supported
		api.GroupToSelectableFields(&api.Group{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "Identity",
		// Ensure all currently returned labels are supported
		api.IdentityToSelectableFields(&api.Identity{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"providerName", "providerUserName", "user.name", "user.uid",
	)

	testutil.CheckFieldLabelConversions(t, "v1", "User",
		// Ensure all currently returned labels are supported
		api.UserToSelectableFields(&api.User{}),
	)
}
