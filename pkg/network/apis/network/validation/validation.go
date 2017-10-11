package validation

import (
	"fmt"
	"net"
	"reflect"

	"k8s.io/apimachinery/pkg/api/validation/path"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/api/validation"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/util/netutils"
)

func validateCIDRv4(cidr string) (*net.IPNet, error) {
	ipnet, err := netutils.ParseCIDRMask(cidr)
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

var defaultClusterNetwork *networkapi.ClusterNetwork

// SetDefaultClusterNetwork sets the expected value of the default ClusterNetwork record
func SetDefaultClusterNetwork(cn networkapi.ClusterNetwork) {
	defaultClusterNetwork = &cn
}

// ValidateClusterNetwork tests if required fields in the ClusterNetwork are set, and ensures that the "default" ClusterNetwork can only be set to the correct values
func ValidateClusterNetwork(clusterNet *networkapi.ClusterNetwork) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&clusterNet.ObjectMeta, false, path.ValidatePathSegmentName, field.NewPath("metadata"))
	var testedCIDRS []*net.IPNet

	if len(clusterNet.Network) != 0 || clusterNet.HostSubnetLength != 0 {
		//In the case that a user manually makes a clusterNetwork object with clusterNet.Network and clusterNet.HostubnetLength at least make sure they are valid values
		clusterIPNet, err := validateCIDRv4(clusterNet.Network)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("network"), clusterNet.Network, err.Error()))
		} else {
			maskLen, addrLen := clusterIPNet.Mask.Size()
			if clusterNet.HostSubnetLength > uint32(addrLen-maskLen) {
				allErrs = append(allErrs, field.Invalid(field.NewPath("hostSubnetLength"), clusterNet.HostSubnetLength, "subnet length is too large for clusterNetwork"))
			} else if clusterNet.HostSubnetLength < 2 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("hostSubnetLength"), clusterNet.HostSubnetLength, "subnet length must be at least 2"))
			}
		}
	}

	if len(clusterNet.ClusterNetworks) == 0 && len(clusterNet.Network) == 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("clusterNetworks"), clusterNet.ClusterNetworks, "must have at least one cluster network CIDR"))
	}
	serviceIPNet, err := validateCIDRv4(clusterNet.ServiceNetwork)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("serviceNetwork"), clusterNet.ServiceNetwork, err.Error()))
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
			if configapi.CIDRsOverlap(clusterIPNet.String(), cidr.String()) {
				allErrs = append(allErrs, field.Invalid(field.NewPath("clusterNetworks").Index(i).Child("cidr"), cn.CIDR, fmt.Sprintf("cidr range overlaps with another cidr %q", cidr.String())))
			}
		}
		testedCIDRS = append(testedCIDRS, clusterIPNet)

		if (clusterIPNet != nil) && (serviceIPNet != nil) && configapi.CIDRsOverlap(clusterIPNet.String(), serviceIPNet.String()) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("serviceNetwork"), clusterNet.ServiceNetwork, fmt.Sprintf("service network overlaps with cluster network cidr: %s", clusterIPNet.String())))
		}
	}

	if clusterNet.Name == networkapi.ClusterNetworkDefault && defaultClusterNetwork != nil {
		if clusterNet.Network != defaultClusterNetwork.Network {
			allErrs = append(allErrs, field.Invalid(field.NewPath("network"), clusterNet.Network, "cannot change the default ClusterNetwork record via API."))
		}
		if clusterNet.HostSubnetLength != defaultClusterNetwork.HostSubnetLength {
			allErrs = append(allErrs, field.Invalid(field.NewPath("hostSubnetLength"), clusterNet.HostSubnetLength, "cannot change the default ClusterNetwork record via API."))
		}
		if !reflect.DeepEqual(clusterNet.ClusterNetworks, defaultClusterNetwork.ClusterNetworks) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("ClusterNetworks"), clusterNet.ClusterNetworks, "cannot change the default ClusterNetwork record via API"))
		}
		if clusterNet.ServiceNetwork != defaultClusterNetwork.ServiceNetwork {
			allErrs = append(allErrs, field.Invalid(field.NewPath("serviceNetwork"), clusterNet.ServiceNetwork, "cannot change the default ClusterNetwork record via API."))
		}
		if clusterNet.PluginName != defaultClusterNetwork.PluginName {
			allErrs = append(allErrs, field.Invalid(field.NewPath("pluginName"), clusterNet.PluginName, "cannot change the default ClusterNetwork record via API."))
		}
	}

	return allErrs
}

func ValidateClusterNetworkUpdate(obj *networkapi.ClusterNetwork, old *networkapi.ClusterNetwork) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateClusterNetwork(obj)...)
	return allErrs
}

func ValidateHostSubnet(hs *networkapi.HostSubnet) field.ErrorList {
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

	return allErrs
}

func ValidateHostSubnetUpdate(obj *networkapi.HostSubnet, old *networkapi.HostSubnet) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateHostSubnet(obj)...)

	if obj.Subnet != old.Subnet {
		allErrs = append(allErrs, field.Invalid(field.NewPath("subnet"), obj.Subnet, "cannot change the subnet lease midflight."))
	}

	return allErrs
}

// ValidateNetNamespace tests fields for a greater-than-zero NetID
func ValidateNetNamespace(netnamespace *networkapi.NetNamespace) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&netnamespace.ObjectMeta, false, path.ValidatePathSegmentName, field.NewPath("metadata"))

	if netnamespace.NetName != netnamespace.Name {
		allErrs = append(allErrs, field.Invalid(field.NewPath("netname"), netnamespace.NetName, fmt.Sprintf("must be the same as metadata.name: %q", netnamespace.Name)))
	}

	if err := network.ValidVNID(netnamespace.NetID); err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("netid"), netnamespace.NetID, err.Error()))
	}

	for i, ip := range netnamespace.EgressIPs {
		if _, err := validateIPv4(ip); err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("egressIPs").Index(i), ip, err.Error()))
		}
	}

	return allErrs
}

func ValidateNetNamespaceUpdate(obj *networkapi.NetNamespace, old *networkapi.NetNamespace) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateNetNamespace(obj)...)
	return allErrs
}

// ValidateEgressNetworkPolicy tests if required fields in the EgressNetworkPolicy are set.
func ValidateEgressNetworkPolicy(policy *networkapi.EgressNetworkPolicy) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&policy.ObjectMeta, true, path.ValidatePathSegmentName, field.NewPath("metadata"))

	for i, rule := range policy.Spec.Egress {
		if rule.Type != networkapi.EgressNetworkPolicyRuleAllow && rule.Type != networkapi.EgressNetworkPolicyRuleDeny {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("egress").Index(i).Child("type"), rule.Type, "invalid policy type"))
		}

		if len(rule.To.CIDRSelector) == 0 && len(rule.To.DNSName) == 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("egress").Index(i).Child("to"), rule.To, "must specify cidrSelector or dnsName"))
		} else if len(rule.To.CIDRSelector) != 0 && len(rule.To.DNSName) != 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("egress").Index(i).Child("to"), rule.To, "either specify cidrSelector or dnsName but not both"))
		}

		if len(rule.To.CIDRSelector) > 0 {
			if _, err := netutils.ParseCIDRMask(rule.To.CIDRSelector); err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("egress").Index(i).Child("to", "cidrSelector"), rule.To.CIDRSelector, err.Error()))
			}
		}

		if len(rule.To.DNSName) > 0 {
			if len(utilvalidation.IsDNS1123Subdomain(rule.To.DNSName)) != 0 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("egress").Index(i).Child("to", "dnsName"), rule.To.DNSName, "must conform to DNS 952 subdomain conventions"))
			}
		}
	}

	if len(policy.Spec.Egress) > networkapi.EgressNetworkPolicyMaxRules {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("egress"), "", ("too many egress rules (max 50)")))
	}

	return allErrs
}

func ValidateEgressNetworkPolicyUpdate(obj *networkapi.EgressNetworkPolicy, old *networkapi.EgressNetworkPolicy) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateEgressNetworkPolicy(obj)...)
	return allErrs
}
