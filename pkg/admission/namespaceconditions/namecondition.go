package namespaceconditions

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
)

// pluginHandlerWithNamespaceNameConditions skips running admission plugins if they deal in the namespaceToExclude list
type pluginHandlerWithNamespaceNameConditions struct {
	admissionPlugin     admission.Interface
	namespacesToExclude sets.String
}

func (p pluginHandlerWithNamespaceNameConditions) Handles(operation admission.Operation) bool {
	return p.admissionPlugin.Handles(operation)
}

// Admit performs a mutating admission control check and emit metrics.
func (p pluginHandlerWithNamespaceNameConditions) Admit(a admission.Attributes) error {
	if !p.shouldRunAdmission(a) {
		return nil
	}

	mutatingHandler, ok := p.admissionPlugin.(admission.MutationInterface)
	if !ok {
		return nil
	}
	return mutatingHandler.Admit(a)
}

// Validate performs a non-mutating admission control check and emits metrics.
func (p pluginHandlerWithNamespaceNameConditions) Validate(a admission.Attributes) error {
	if !p.shouldRunAdmission(a) {
		return nil
	}

	validatingHandler, ok := p.admissionPlugin.(admission.ValidationInterface)
	if !ok {
		return nil
	}
	return validatingHandler.Validate(a)
}

func (p pluginHandlerWithNamespaceNameConditions) shouldRunAdmission(attr admission.Attributes) bool {
	namespaceName := attr.GetNamespace()
	if p.namespacesToExclude.Has(namespaceName) {
		return false
	}
	if (attr.GetResource().GroupResource() == schema.GroupResource{Resource: "namespaces"}) && p.namespacesToExclude.Has(attr.GetName()) {
		return false
	}

	return true
}
