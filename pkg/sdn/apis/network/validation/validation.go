package validation

import (
	"fmt"
	"net"

	"k8s.io/apimachinery/pkg/api/validation/path"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/api/validation"

	"github.com/openshift/origin/pkg/sdn"
	sdnapi "github.com/openshift/origin/pkg/sdn/apis/network"
	"github.com/openshift/origin/pkg/util/netutils"
)

var defaultClusterNetwork *sdnapi.ClusterNetwork

// SetDefaultClusterNetwork sets the expected value of the default ClusterNetwork record
func SetDefaultClusterNetwork(cn sdnapi.ClusterNetwork) {
	defaultClusterNetwork = &cn
}

// ValidateClusterNetwork tests if required fields in the ClusterNetwork are set, and ensures that the "default" ClusterNetwork can only be set to the correct values
func ValidateClusterNetwork(clusterNet *sdnapi.ClusterNetwork) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&clusterNet.ObjectMeta, false, path.ValidatePathSegmentName, field.NewPath("metadata"))

	clusterIPNet, err := netutils.ParseCIDRMask(clusterNet.Network)
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

	serviceIPNet, err := netutils.ParseCIDRMask(clusterNet.ServiceNetwork)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("serviceNetwork"), clusterNet.ServiceNetwork, err.Error()))
	}

	if (clusterIPNet != nil) && (serviceIPNet != nil) && clusterIPNet.Contains(serviceIPNet.IP) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("serviceNetwork"), clusterNet.ServiceNetwork, "service network overlaps with cluster network"))
	}
	if (serviceIPNet != nil) && (clusterIPNet != nil) && serviceIPNet.Contains(clusterIPNet.IP) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("network"), clusterNet.Network, "cluster network overlaps with service network"))
	}

	if clusterNet.Name == sdnapi.ClusterNetworkDefault && defaultClusterNetwork != nil {
		if clusterNet.Network != defaultClusterNetwork.Network {
			allErrs = append(allErrs, field.Invalid(field.NewPath("network"), clusterNet.Network, "cannot change the default ClusterNetwork record via API."))
		}
		if clusterNet.HostSubnetLength != defaultClusterNetwork.HostSubnetLength {
			allErrs = append(allErrs, field.Invalid(field.NewPath("hostSubnetLength"), clusterNet.HostSubnetLength, "cannot change the default ClusterNetwork record via API."))
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

func ValidateClusterNetworkUpdate(obj *sdnapi.ClusterNetwork, old *sdnapi.ClusterNetwork) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateClusterNetwork(obj)...)
	return allErrs
}

// ValidateHostSubnet tests fields for the host subnet, the host should be a network resolvable string,
//  and subnet should be a valid CIDR
func ValidateHostSubnet(hs *sdnapi.HostSubnet) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&hs.ObjectMeta, false, path.ValidatePathSegmentName, field.NewPath("metadata"))

	if hs.Host != hs.Name {
		allErrs = append(allErrs, field.Invalid(field.NewPath("host"), hs.Host, fmt.Sprintf("must be the same as metadata.name: %q", hs.Name)))
	}

	if hs.Subnet == "" {
		// check if annotation exists, then let the Subnet field be empty
		if _, ok := hs.Annotations[sdnapi.AssignHostSubnetAnnotation]; !ok {
			allErrs = append(allErrs, field.Invalid(field.NewPath("subnet"), hs.Subnet, "field cannot be empty"))
		}
	} else {
		_, err := netutils.ParseCIDRMask(hs.Subnet)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("subnet"), hs.Subnet, err.Error()))
		}
	}
	if net.ParseIP(hs.HostIP) == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("hostIP"), hs.HostIP, "invalid IP address"))
	}
	return allErrs
}

func ValidateHostSubnetUpdate(obj *sdnapi.HostSubnet, old *sdnapi.HostSubnet) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateHostSubnet(obj)...)

	if obj.Subnet != old.Subnet {
		allErrs = append(allErrs, field.Invalid(field.NewPath("subnet"), obj.Subnet, "cannot change the subnet lease midflight."))
	}

	return allErrs
}

// ValidateNetNamespace tests fields for a greater-than-zero NetID
func ValidateNetNamespace(netnamespace *sdnapi.NetNamespace) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&netnamespace.ObjectMeta, false, path.ValidatePathSegmentName, field.NewPath("metadata"))

	if netnamespace.NetName != netnamespace.Name {
		allErrs = append(allErrs, field.Invalid(field.NewPath("netname"), netnamespace.NetName, fmt.Sprintf("must be the same as metadata.name: %q", netnamespace.Name)))
	}

	if err := sdn.ValidVNID(netnamespace.NetID); err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("netid"), netnamespace.NetID, err.Error()))
	}
	return allErrs
}

func ValidateNetNamespaceUpdate(obj *sdnapi.NetNamespace, old *sdnapi.NetNamespace) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateNetNamespace(obj)...)
	return allErrs
}

// ValidateEgressNetworkPolicy tests if required fields in the EgressNetworkPolicy are set.
func ValidateEgressNetworkPolicy(policy *sdnapi.EgressNetworkPolicy) field.ErrorList {
	allErrs := validation.ValidateObjectMeta(&policy.ObjectMeta, true, path.ValidatePathSegmentName, field.NewPath("metadata"))

	for i, rule := range policy.Spec.Egress {
		if rule.Type != sdnapi.EgressNetworkPolicyRuleAllow && rule.Type != sdnapi.EgressNetworkPolicyRuleDeny {
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

	if len(policy.Spec.Egress) > sdnapi.EgressNetworkPolicyMaxRules {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("egress"), "", ("too many egress rules (max 50)")))
	}

	return allErrs
}

func ValidateEgressNetworkPolicyUpdate(obj *sdnapi.EgressNetworkPolicy, old *sdnapi.EgressNetworkPolicy) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateEgressNetworkPolicy(obj)...)
	return allErrs
}
