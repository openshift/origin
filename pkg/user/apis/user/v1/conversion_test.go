package v1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/api/apihelpers/apitesting"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

func TestFieldSelectorConversions(t *testing.T) {
	converter := runtime.NewScheme()
	LegacySchemeBuilder.AddToScheme(converter)

	apitesting.TestFieldLabelConversions(t, converter, "v1", "Identity",
		// Ensure all currently returned labels are supported
		userapi.IdentityToSelectableFields(&userapi.Identity{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"providerName", "providerUserName", "user.name", "user.uid",
	)
}
