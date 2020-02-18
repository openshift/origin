package resourcemerge

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
)

// EnsureCustomResourceDefinitionV1Beta1 ensures that the existing matches the required.
// modified is set to true when existing had to be updated with required.
func EnsureCustomResourceDefinitionV1Beta1(modified *bool, existing *apiextensionsv1beta1.CustomResourceDefinition, required apiextensionsv1beta1.CustomResourceDefinition) {
	EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	// we stomp everything
	if !equality.Semantic.DeepEqual(existing.Spec, required.Spec) {
		*modified = true
		existing.Spec = required.Spec
	}
}

// EnsureCustomResourceDefinitionV1 ensures that the existing matches the required.
// modified is set to true when existing had to be updated with required.
func EnsureCustomResourceDefinitionV1(modified *bool, existing *apiextensionsv1.CustomResourceDefinition, required apiextensionsv1.CustomResourceDefinition) {
	EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	// we stomp everything
	if !equality.Semantic.DeepEqual(existing.Spec, required.Spec) {
		*modified = true
		existing.Spec = required.Spec
	}
}
