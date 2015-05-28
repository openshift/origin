package validation

import (
	"net"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	sdnapi "github.com/openshift/origin/pkg/sdn/api"
)

// ValidateClusterNetwork tests if required fields in the ClusterNetwork are set.
func ValidateClusterNetwork(clusterNet *sdnapi.ClusterNetwork) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	_, ipnet, err := net.ParseCIDR(clusterNet.Network)
	if err != nil {
		result = append(result, fielderrors.NewFieldInvalid("network", clusterNet.Network, err.Error()))
	} else {
		ones, bitSize := ipnet.Mask.Size()
		if (bitSize - ones) <= clusterNet.HostSubnetLength {
			result = append(result, fielderrors.NewFieldInvalid("hostSubnetLength", clusterNet.HostSubnetLength, "subnet length is greater than cluster Mask"))
		}
	}

	return result
}

// ValidateHostSubnet tests fields for the host subnet, the host should be a network resolvable string,
//  and subnet should be a valid CIDR
func ValidateHostSubnet(hs *sdnapi.HostSubnet) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}
	if hs.Name == "" {
		result = append(result, fielderrors.NewFieldInvalid("name", hs.Name, "name missing in object metadata"))
	}
	_, _, err := net.ParseCIDR(hs.Subnet)
	if err != nil {
		result = append(result, fielderrors.NewFieldInvalid("subnet", hs.Subnet, err.Error()))
	}
	if net.ParseIP(hs.HostIP) == nil {
		result = append(result, fielderrors.NewFieldInvalid("hostIP", hs.HostIP, "invalid IP address"))
	}
	return result
}
