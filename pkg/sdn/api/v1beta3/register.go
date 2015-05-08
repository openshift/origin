package v1beta3

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta3",
		&ClusterNetwork{},
		&HostSubnet{},
		&HostSubnetList{},
	)
}

func (*ClusterNetwork) IsAnAPIObject() {}
func (*HostSubnet) IsAnAPIObject()     {}
func (*HostSubnetList) IsAnAPIObject() {}
