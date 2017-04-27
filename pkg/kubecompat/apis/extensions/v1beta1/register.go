package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kapi "k8s.io/kubernetes/pkg/api"
)

const GroupName = "extensions"

var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1beta1"}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes, addConversionFuncs)
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(
		SchemeGroupVersion,
		&HorizontalPodAutoscaler{},
		&HorizontalPodAutoscalerList{},
	)
	return nil
}

func init() {
	if err := SchemeBuilder.AddToScheme(kapi.Scheme); err != nil {
		panic(err)
	}
}
