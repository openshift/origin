package v1

import (
	kapi "k8s.io/kubernetes/pkg/api"

	oapi "github.com/openshift/origin/pkg/api"
	newer "github.com/openshift/origin/pkg/sdn/api"
)

func init() {
	if err := kapi.Scheme.AddFieldLabelConversionFunc("v1", "ClusterNetwork",
		oapi.GetFieldLabelConversionFunc(newer.ClusterNetworkToSelectableFields(&newer.ClusterNetwork{}), nil),
	); err != nil {
		panic(err)
	}

	if err := kapi.Scheme.AddFieldLabelConversionFunc("v1", "HostSubnet",
		oapi.GetFieldLabelConversionFunc(newer.HostSubnetToSelectableFields(&newer.HostSubnet{}), nil),
	); err != nil {
		panic(err)
	}

	if err := kapi.Scheme.AddFieldLabelConversionFunc("v1", "NetNamespace",
		oapi.GetFieldLabelConversionFunc(newer.NetNamespaceToSelectableFields(&newer.NetNamespace{}), nil),
	); err != nil {
		panic(err)
	}
}
