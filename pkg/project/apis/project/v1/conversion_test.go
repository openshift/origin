package v1

import (
	"testing"

	"github.com/openshift/origin/pkg/api/apihelpers/apitesting"

	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/registry/core/namespace"
)

func TestFieldSelectorConversions(t *testing.T) {
	converter := runtime.NewScheme()
	LegacySchemeBuilder.AddToScheme(converter)

	apitesting.TestFieldLabelConversions(t, converter, "v1", "Project",
		// Ensure all currently returned labels are supported
		namespace.NamespaceToSelectableFields(&kapi.Namespace{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"status.phase",
	)
}
