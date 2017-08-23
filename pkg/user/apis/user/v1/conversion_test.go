package v1_test

import (
	"testing"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
	testutil "github.com/openshift/origin/test/util/api"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "Identity",
		// Ensure all currently returned labels are supported
		userapi.IdentityToSelectableFields(&userapi.Identity{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"providerName", "providerUserName", "user.name", "user.uid",
	)

}
