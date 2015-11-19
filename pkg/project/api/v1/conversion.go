package v1

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/registry/namespace"

	oapi "github.com/openshift/origin/pkg/api"
)

func init() {
	if err := kapi.Scheme.AddFieldLabelConversionFunc("v1", "Project",
		oapi.GetFieldLabelConversionFunc(namespace.NamespaceToSelectableFields(&kapi.Namespace{}), nil),
	); err != nil {
		panic(err)
	}
}
