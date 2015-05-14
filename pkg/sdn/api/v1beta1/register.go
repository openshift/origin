package v1beta1

import (
	api "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta1",
		&ClusterNetwork{},
		&HostSubnet{},
		&HostSubnetList{},
	)
}

func (*ClusterNetwork) IsAnAPIObject() {}
func (*HostSubnet) IsAnAPIObject()     {}
func (*HostSubnetList) IsAnAPIObject() {}
