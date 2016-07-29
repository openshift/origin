package upstreamconversions

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	extensionsapi "k8s.io/kubernetes/pkg/apis/extensions"
	extensionsv1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"
)

func AddToScheme(scheme *runtime.Scheme) {
	addConversionFuncs(scheme)
}

func addConversionFuncs(scheme *runtime.Scheme) {
	if err := scheme.AddConversionFuncs(
		Convert_v1beta1_ReplicaSet_to_api_ReplicationController,
	); err != nil {
		panic(err)
	}
}

func Convert_v1beta1_ReplicaSet_to_api_ReplicationController(in *extensionsv1beta1.ReplicaSet, out *kapi.ReplicationController, s conversion.Scope) error {
	intermediate1 := &extensionsapi.ReplicaSet{}
	if err := extensionsv1beta1.Convert_v1beta1_ReplicaSet_To_extensions_ReplicaSet(in, intermediate1, s); err != nil {
		return err
	}

	intermediate2 := &kapiv1.ReplicationController{}
	if err := kapiv1.Convert_extensions_ReplicaSet_to_v1_ReplicationController(intermediate1, intermediate2, s); err != nil {
		return err
	}

	return kapiv1.Convert_v1_ReplicationController_To_api_ReplicationController(intermediate2, out, s)
}
