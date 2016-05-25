package v1

import (
	"reflect"

	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"

	internal "github.com/openshift/origin/pkg/quota/api"
)

func Convert_v1_ResourceQuotasStatusByNamespace_To_api_ResourceQuotasStatusByNamespace(in *ResourceQuotasStatusByNamespace, out *internal.ResourceQuotasStatusByNamespace, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*ResourceQuotasStatusByNamespace))(in)
	}

	if in == nil {
		return nil
	}

	for _, curr := range *in {
		internalStatus := &kapi.ResourceQuotaStatus{}
		kapiv1.Convert_v1_ResourceQuotaStatus_To_api_ResourceQuotaStatus(&curr.Status, internalStatus, s)

		out.Insert(curr.Namespace, *internalStatus)
	}

	return nil
}

func Convert_api_ResourceQuotasStatusByNamespace_To_v1_ResourceQuotasStatusByNamespace(in *internal.ResourceQuotasStatusByNamespace, out *ResourceQuotasStatusByNamespace, s conversion.Scope) error {
	for e := in.OrderedKeys().Front(); e != nil; e = e.Next() {
		namespace := e.Value.(string)
		status, _ := in.Get(namespace)

		versionedStatus := &kapiv1.ResourceQuotaStatus{}
		kapiv1.Convert_api_ResourceQuotaStatus_To_v1_ResourceQuotaStatus(&status, versionedStatus, s)

		if out == nil {
			out = &ResourceQuotasStatusByNamespace{}
		}
		*out = append(*out, ResourceQuotaStatusByNamespace{Namespace: namespace, Status: *versionedStatus})
	}

	return nil
}

func addConversionFuncs(scheme *runtime.Scheme) {
	err := scheme.AddConversionFuncs(
		Convert_api_ResourceQuotasStatusByNamespace_To_v1_ResourceQuotasStatusByNamespace,
		Convert_v1_ResourceQuotasStatusByNamespace_To_api_ResourceQuotasStatusByNamespace,
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}

}
