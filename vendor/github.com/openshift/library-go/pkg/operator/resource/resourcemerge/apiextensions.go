package resourcemerge

import (
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
)

// EnsureCustomResourceDefinition ensures that the existing matches the required.
// modified is set to true when existing had to be updated with required.
func EnsureCustomResourceDefinition(modified *bool, existing *apiextv1beta1.CustomResourceDefinition, required apiextv1beta1.CustomResourceDefinition) {
	EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	// we stomp everything
	if !equality.Semantic.DeepEqual(existing.Spec, required.Spec) {
		*modified = true
		existing.Spec = required.Spec
	}
}
