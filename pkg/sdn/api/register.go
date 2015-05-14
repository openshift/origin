package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
		&ClusterNetwork{},
		&HostSubnet{},
		&HostSubnetList{},
	)
}

func (*ClusterNetwork) IsAnAPIObject() {}
func (*HostSubnet) IsAnAPIObject()     {}
func (*HostSubnetList) IsAnAPIObject() {}
