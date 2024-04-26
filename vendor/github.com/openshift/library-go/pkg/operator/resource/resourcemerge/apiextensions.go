package resourcemerge

import (
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"
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

	// we need to match defaults
	mimicCRDV1Defaulting(&required)
	// we stomp everything
	if !equality.Semantic.DeepEqual(existing.Spec, required.Spec) {
		*modified = true
		existing.Spec = required.Spec
	}
}

func mimicCRDV1Defaulting(required *apiextensionsv1.CustomResourceDefinition) {
	crd_SetDefaults_CustomResourceDefinitionSpec(&required.Spec)

	if required.Spec.Conversion != nil &&
		required.Spec.Conversion.Webhook != nil &&
		required.Spec.Conversion.Webhook.ClientConfig != nil &&
		required.Spec.Conversion.Webhook.ClientConfig.Service != nil {
		crd_SetDefaults_ServiceReference(required.Spec.Conversion.Webhook.ClientConfig.Service)
	}
}

// lifted from https://github.com/kubernetes/kubernetes/blob/v1.21.0/staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1/defaults.go#L42-L61
func crd_SetDefaults_CustomResourceDefinitionSpec(obj *apiextensionsv1.CustomResourceDefinitionSpec) {
	if len(obj.Names.Singular) == 0 {
		obj.Names.Singular = strings.ToLower(obj.Names.Kind)
	}
	if len(obj.Names.ListKind) == 0 && len(obj.Names.Kind) > 0 {
		obj.Names.ListKind = obj.Names.Kind + "List"
	}
	if obj.Conversion == nil {
		obj.Conversion = &apiextensionsv1.CustomResourceConversion{
			Strategy: apiextensionsv1.NoneConverter,
		}
	}
}

func crd_SetDefaults_ServiceReference(obj *apiextensionsv1.ServiceReference) {
	if obj.Port == nil {
		obj.Port = ptr.To[int32](443)
	}
}
