package v1

import (
	"k8s.io/apimachinery/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	sdnapi "github.com/openshift/origin/pkg/sdn/apis/network"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc("network.openshift.io/v1", "ClusterNetwork",
		oapi.GetFieldLabelConversionFunc(sdnapi.ClusterNetworkToSelectableFields(&sdnapi.ClusterNetwork{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("network.openshift.io/v1", "HostSubnet",
		oapi.GetFieldLabelConversionFunc(sdnapi.HostSubnetToSelectableFields(&sdnapi.HostSubnet{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("network.openshift.io/v1", "NetNamespace",
		oapi.GetFieldLabelConversionFunc(sdnapi.NetNamespaceToSelectableFields(&sdnapi.NetNamespace{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("network.openshift.io/v1", "EgressNetworkPolicy",
		oapi.GetFieldLabelConversionFunc(sdnapi.EgressNetworkPolicyToSelectableFields(&sdnapi.EgressNetworkPolicy{}), nil),
	); err != nil {
		return err
	}

	return addLegacyConversionFuncs(scheme)
}

func addLegacyConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc("v1", "ClusterNetwork",
		oapi.GetFieldLabelConversionFunc(sdnapi.ClusterNetworkToSelectableFields(&sdnapi.ClusterNetwork{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "HostSubnet",
		oapi.GetFieldLabelConversionFunc(sdnapi.HostSubnetToSelectableFields(&sdnapi.HostSubnet{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "NetNamespace",
		oapi.GetFieldLabelConversionFunc(sdnapi.NetNamespaceToSelectableFields(&sdnapi.NetNamespace{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "EgressNetworkPolicy",
		oapi.GetFieldLabelConversionFunc(sdnapi.EgressNetworkPolicyToSelectableFields(&sdnapi.EgressNetworkPolicy{}), nil),
	); err != nil {
		return err
	}
	return nil
}
