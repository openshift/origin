package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/registry/core/namespace"

	oapi "github.com/openshift/origin/pkg/api"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc("v1", "Project",
		oapi.GetFieldLabelConversionFunc(namespace.NamespaceToSelectableFields(&kapi.Namespace{}), nil),
	); err != nil {
		return err
	}
	return scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.String(), "Project",
		oapi.GetFieldLabelConversionFunc(namespace.NamespaceToSelectableFields(&kapi.Namespace{}), nil),
	)
}
