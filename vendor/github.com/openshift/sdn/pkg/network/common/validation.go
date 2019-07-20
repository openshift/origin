package common

import (
	"fmt"
	"net"

	"k8s.io/apimachinery/pkg/api/validation/path"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core/validation"

	networkapi "github.com/openshift/api/network/v1"
	"github.com/openshift/library-go/pkg/network/networkutils"
)

func validateCIDRv4(cidr string) (*net.IPNet, error) {
	ipnet, err := networkutils.ParseCIDRMask(cidr)
	if err != nil {
		return nil, err
	}
	if ipnet.IP.To4() == nil {
		return nil, fmt.Errorf("must be an IPv4 network")
	}
	return ipnet, nil
}

func validateIPv4(ip string) (net.IP, error) {
	bytes := net.ParseIP(ip)
	if bytes == nil {
		return nil, fmt.Errorf("invalid IP address")
	}
	if bytes.To4() == nil {
		return nil, fmt.Errorf("must be an IPv4 address")
	}
	return bytes, nil
}

// ValidateClusterNetwork tests if required fields in the ClusterNetwork are set, and ensures that the "default" ClusterNetwork can only be set to the correct values
func ValidateClusterNetwork(clusterNet *networkapi.ClusterNetwork) error {
	allErrs := validation.ValidateObjectMeta(&clusterNet.ObjectMeta, false, path.ValidatePathSegmentName, field.NewPath("metadata"))
	var testedCIDRS []*net.IPNet

	serviceIPNet, err := validateCIDRv4(clusterNet.ServiceNetwork)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("serviceNetwork"), clusterNet.ServiceNetwork, err.Error()))
	}

	if len(clusterNet.ClusterNetworks) == 0 {
		// legacy ClusterNetwork; old fields must be set
		if clusterNet.Network == "" {
			allErrs = append(allErrs, field.Required(field.NewPath("network"), "network must be set (if clusterNetworks is empty)"))
		} else if clusterNet.HostSubnetLength == 0 {
			allErrs = append(allErrs, field.Required(field.NewPath("hostsubnetlength"), "hostsubnetlength must be set (if clusterNetworks is empty)"))
		} else {
			clusterIPNet, err := validateCIDRv4(clusterNet.Network)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("network"), clusterNet.Network, err.Error()))
			}
			maskLen, addrLen := clusterIPNet.Mask.Size()
			if clusterNet.HostSubnetLength > uint32(addrLen-maskLen) {
				allErrs = append(allErrs, field.Invalid(field.NewPath("hostsubnetlength"), clusterNet.HostSubnetLength, "subnet length is too large for cidr"))
			} else if clusterNet.HostSubnetLength < 2 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("hostsubnetlength"), clusterNet.HostSubnetLength, "subnet length must be at least 2"))
			}

			if (clusterIPNet != nil) && (serviceIPNet != nil) && CIDRsOverlap(clusterIPNet.String(), serviceIPNet.String()) {
				allErrs = append(allErrs, field.Invalid(field.NewPath("serviceNetwork"), clusterNet.ServiceNetwork, "service network overlaps with cluster network"))
			}
		}
	} else {
		// "new" ClusterNetwork
		if clusterNet.Name == networkapi.ClusterNetworkDefault {
			if clusterNet.Network != clusterNet.ClusterNetworks[0].CIDR {
				allErrs = append(allErrs, field.Invalid(field.NewPath("network"), clusterNet.Network, "network must be identical to clusterNetworks[0].cidr"))
			}
			if clusterNet.HostSubnetLength != clusterNet.ClusterNetworks[0].HostSubnetLength {
				allErrs = append(allErrs, field.Invalid(field.NewPath("hostsubnetlength"), clusterNet.HostSubnetLength, "hostsubnetlength must be identical to clusterNetworks[0].hostSubnetLength"))
			}
		} else if clusterNet.Network != "" || clusterNet.HostSubnetLength != 0 {
			if clusterNet.Network != clusterNet.ClusterNetworks[0].CIDR || clusterNet.HostSubnetLength != clusterNet.ClusterNetworks[0].HostSubnetLength {
				allErrs = append(allErrs, field.Invalid(field.NewPath("clusterNetworks").Index(0), clusterNet.ClusterNetworks[0], "network and hostsubnetlength must be unset or identical to clusterNetworks[0]"))
			}
		}
	}

	for i, cn := range clusterNet.ClusterNetworks {
		clusterIPNet, err := validateCIDRv4(cn.CIDR)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("clusterNetworks").Index(i).Child("cidr"), cn.CIDR, err.Error()))
			continue
		}
		maskLen, addrLen := clusterIPNet.Mask.Size()
		if cn.HostSubnetLength > uint32(addrLen-maskLen) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("clusterNetworks").Index(i).Child("hostSubnetLength"), cn.HostSubnetLength, "subnet length is too large for clusterNetwork "))
		} else if cn.HostSubnetLength < 2 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("clusterNetworks").Index(i).Child("hostSubnetLength"), cn.HostSubnetLength, "subnet length must be at least 2"))
		}

		for _, cidr := range testedCIDRS {
			if CIDRsOverlap(clusterIPNet.String(), cidr.String()) {
				allErrs = append(allErrs, field.Invalid(field.NewPath("clusterNetworks").Index(i).Child("cidr"), cn.CIDR, fmt.Sprintf("cidr range overlaps with another cidr %q", cidr.String())))
			}
		}
		testedCIDRS = append(testedCIDRS, clusterIPNet)

		if (clusterIPNet != nil) && (serviceIPNet != nil) && CIDRsOverlap(clusterIPNet.String(), serviceIPNet.String()) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("serviceNetwork"), clusterNet.ServiceNetwork, fmt.Sprintf("service network overlaps with cluster network cidr: %s", clusterIPNet.String())))
		}
	}

	if clusterNet.VXLANPort != nil {
		for _, msg := range utilvalidation.IsValidPortNum(int(*clusterNet.VXLANPort)) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("vxlanPort"), clusterNet.VXLANPort, msg))
		}
	}

	if len(allErrs) > 0 {
		return allErrs.ToAggregate()
	} else {
		return nil
	}
}

func ValidateHostSubnet(hs *networkapi.HostSubnet) error {
	allErrs := validation.ValidateObjectMeta(&hs.ObjectMeta, false, path.ValidatePathSegmentName, field.NewPath("metadata"))

	if hs.Host != hs.Name {
		allErrs = append(allErrs, field.Invalid(field.NewPath("host"), hs.Host, fmt.Sprintf("must be the same as metadata.name: %q", hs.Name)))
	}

	if hs.Subnet == "" {
		// check if annotation exists, then let the Subnet field be empty
		if _, ok := hs.Annotations[networkapi.AssignHostSubnetAnnotation]; !ok {
			allErrs = append(allErrs, field.Invalid(field.NewPath("subnet"), hs.Subnet, "field cannot be empty"))
		}
	} else {
		_, err := validateCIDRv4(hs.Subnet)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("subnet"), hs.Subnet, err.Error()))
		}
	}
	// In theory this has to be IPv4, but it's possible some clusters might be limping along with IPv6 values?
	if net.ParseIP(hs.HostIP) == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("hostIP"), hs.HostIP, "invalid IP address"))
	}

	for i, egressIP := range hs.EgressIPs {
		if _, err := validateIPv4(egressIP); err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("egressIPs").Index(i), egressIP, err.Error()))
		}
	}

	for i, egressCIDR := range hs.EgressCIDRs {
		if _, err := validateCIDRv4(egressCIDR); err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("egressCIDRs").Index(i), egressCIDR, err.Error()))
		}
	}

	if len(allErrs) > 0 {
		return allErrs.ToAggregate()
	} else {
		return nil
	}
}

func CIDRsOverlap(cidr1, cidr2 string) bool {
	_, ipNet1, err := net.ParseCIDR(cidr1)
	if err != nil {
		return false
	}
	_, ipNet2, err := net.ParseCIDR(cidr2)
	if err != nil {
		return false
	}
	return ipNet1.Contains(ipNet2.IP) || ipNet2.Contains(ipNet1.IP)
}
