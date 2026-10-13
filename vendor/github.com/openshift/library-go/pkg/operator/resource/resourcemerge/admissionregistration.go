package resourcemerge

import (
	operatorsv1 "github.com/openshift/api/operator/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ExpectedMutatingWebhooksConfiguration returns last applied generation for MutatingWebhookConfiguration resource registered in operator
func ExpectedMutatingWebhooksConfiguration(name string, previousGenerations []operatorsv1.GenerationStatus) int64 {
	generation := GenerationFor(previousGenerations, schema.GroupResource{Group: admissionregistrationv1.SchemeGroupVersion.Group, Resource: "mutatingwebhookconfigurations"}, "", name)
	if generation != nil {
		return generation.LastGeneration
	}
	return -1
}

// SetMutatingWebhooksConfigurationGeneration updates operator generation status list with last applied generation for provided MutatingWebhookConfiguration resource
func SetMutatingWebhooksConfigurationGeneration(generations *[]operatorsv1.GenerationStatus, actual *admissionregistrationv1.MutatingWebhookConfiguration) {
	if actual == nil {
		return
	}
	SetGeneration(generations, operatorsv1.GenerationStatus{
		Group:          admissionregistrationv1.SchemeGroupVersion.Group,
		Resource:       "mutatingwebhookconfigurations",
		Name:           actual.Name,
		LastGeneration: actual.ObjectMeta.Generation,
	})
}

// ExpectedValidatingWebhooksConfiguration returns last applied generation for ValidatingWebhookConfiguration resource registered in operator
func ExpectedValidatingWebhooksConfiguration(name string, previousGenerations []operatorsv1.GenerationStatus) int64 {
	generation := GenerationFor(previousGenerations, schema.GroupResource{Group: admissionregistrationv1.SchemeGroupVersion.Group, Resource: "validatingwebhookconfigurations"}, "", name)
	if generation != nil {
		return generation.LastGeneration
	}
	return -1
}

// SetValidatingWebhooksConfigurationGeneration updates operator generation status list with last applied generation for provided ValidatingWebhookConfiguration resource
func SetValidatingWebhooksConfigurationGeneration(generations *[]operatorsv1.GenerationStatus, actual *admissionregistrationv1.ValidatingWebhookConfiguration) {
	if actual == nil {
		return
	}
	SetGeneration(generations, operatorsv1.GenerationStatus{
		Group:          admissionregistrationv1.SchemeGroupVersion.Group,
		Resource:       "validatingwebhookconfigurations",
		Name:           actual.Name,
		LastGeneration: actual.ObjectMeta.Generation,
	})
}
