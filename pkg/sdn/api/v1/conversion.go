package v1

import (
	"k8s.io/kubernetes/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/sdn/api"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc("v1", "ClusterNetwork",
		oapi.GetFieldLabelConversionFunc(api.ClusterNetworkToSelectableFields(&api.ClusterNetwork{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "HostSubnet",
		oapi.GetFieldLabelConversionFunc(api.HostSubnetToSelectableFields(&api.HostSubnet{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "NetNamespace",
		oapi.GetFieldLabelConversionFunc(api.NetNamespaceToSelectableFields(&api.NetNamespace{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "EgressNetworkPolicy",
		oapi.GetFieldLabelConversionFunc(api.EgressNetworkPolicyToSelectableFields(&api.EgressNetworkPolicy{}), nil),
	); err != nil {
		return err
	}
	return nil
}
