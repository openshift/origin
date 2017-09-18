package v1_test

import (
	"testing"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	_ "github.com/openshift/origin/pkg/authorization/apis/authorization/install"
	testutil "github.com/openshift/origin/test/util/api"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "PolicyBinding",
		// Ensure all currently returned labels are supported
		authorizationapi.PolicyBindingToSelectableFields(&authorizationapi.PolicyBinding{}),
	)

}
