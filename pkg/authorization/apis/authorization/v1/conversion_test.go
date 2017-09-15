package v1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/api/apihelpers/apitesting"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

func TestFieldSelectorConversions(t *testing.T) {
	converter := runtime.NewScheme()
	LegacySchemeBuilder.AddToScheme(converter)

	apitesting.TestFieldLabelConversions(t, converter, "v1", "PolicyBinding",
		// Ensure all currently returned labels are supported
		authorizationapi.PolicyBindingToSelectableFields(&authorizationapi.PolicyBinding{}),
	)

}
