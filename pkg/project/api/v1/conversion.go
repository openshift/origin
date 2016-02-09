package v1

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/registry/namespace"
	"k8s.io/kubernetes/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
)

func addConversionFuncs(scheme *runtime.Scheme) {
	if err := scheme.AddFieldLabelConversionFunc("v1", "Project",
		oapi.GetFieldLabelConversionFunc(namespace.NamespaceToSelectableFields(&kapi.Namespace{}), nil),
	); err != nil {
		panic(err)
	}
}
