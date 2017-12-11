package validation

import (
	"fmt"
	"net"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/validation/field"
	kubeletoptions "k8s.io/kubernetes/cmd/kubelet/app/options"

	"github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/util/netutils"
)

func ValidateNodeConfig(config *api.NodeConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidateInClusterNodeConfig(config, fldPath)
	if bootstrap := config.KubeletArguments["bootstrap-kubeconfig"]; len(bootstrap) > 0 {
		validationResults.AddErrors(ValidateKubeConfig(bootstrap[0], fldPath.Child("kubeletArguments", "bootstrap-kubeconfig"))...)
	} else {
		validationResults.AddErrors(ValidateKubeConfig(config.MasterKubeConfig, fldPath.Child("masterKubeConfig"))...)
	}
	return validationResults
}

func ValidateInClusterNodeConfig(config *api.NodeConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if len(config.NodeName) == 0 {
		validationResults.AddErrors(field.Required(fldPath.Child("nodeName"), ""))
	}

	servingInfoPath := fldPath.Child("servingInfo")
	hasCertDir := len(config.KubeletArguments["cert-dir"]) > 0
	validationResults.Append(ValidateServingInfo(config.ServingInfo, !hasCertDir, servingInfoPath))
	if config.ServingInfo.BindNetwork == "tcp6" {
		validationResults.AddErrors(field.Invalid(servingInfoPath.Child("bindNetwork"), config.ServingInfo.BindNetwork, "tcp6 is not a valid bindNetwork for nodes, must be tcp or tcp4"))
	}

	if len(config.DNSBindAddress) > 0 {
		validationResults.AddErrors(ValidateHostPort(config.DNSBindAddress, fldPath.Child("dnsBindAddress"))...)
	}
	if len(config.DNSIP) > 0 {
		validationResults.AddErrors(ValidateSpecifiedIP(config.DNSIP, fldPath.Child("dnsIP"))...)
	}
	for i, nameserver := range config.DNSNameservers {
		validationResults.AddErrors(ValidateSpecifiedIPPort(nameserver, fldPath.Child("dnsNameservers").Index(i))...)
	}

	validationResults.AddErrors(ValidateImageConfig(config.ImageConfig, fldPath.Child("imageConfig"))...)

	if config.PodManifestConfig != nil {
		validationResults.AddErrors(ValidatePodManifestConfig(config.PodManifestConfig, fldPath.Child("podManifestConfig"))...)
	}

	validationResults.AddErrors(ValidateNetworkConfig(config.NetworkConfig, fldPath.Child("networkConfig"))...)

	validationResults.AddErrors(ValidateDockerConfig(config.DockerConfig, fldPath.Child("dockerConfig"))...)

	validationResults.AddErrors(ValidateNodeAuthConfig(config.AuthConfig, fldPath.Child("authConfig"))...)

	validationResults.AddErrors(ValidateKubeletExtendedArguments(config.KubeletArguments, fldPath.Child("kubeletArguments"))...)

	if _, err := time.ParseDuration(config.IPTablesSyncPeriod); err != nil {
		validationResults.AddErrors(field.Invalid(fldPath.Child("iptablesSyncPeriod"), config.IPTablesSyncPeriod, fmt.Sprintf("unable to parse iptablesSyncPeriod: %v. Examples with correct format: '5s', '1m', '2h22m'", err)))
	}

	validationResults.AddErrors(ValidateVolumeConfig(config.VolumeConfig, fldPath.Child("volumeConfig"))...)

	return validationResults
}

func ValidateNodeAuthConfig(config api.NodeAuthConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	authenticationCacheTTLPath := fldPath.Child("authenticationCacheTTL")
	if len(config.AuthenticationCacheTTL) == 0 {
		allErrs = append(allErrs, field.Required(authenticationCacheTTLPath, ""))
	} else if ttl, err := time.ParseDuration(config.AuthenticationCacheTTL); err != nil {
		allErrs = append(allErrs, field.Invalid(authenticationCacheTTLPath, config.AuthenticationCacheTTL, fmt.Sprintf("%v", err)))
	} else if ttl < 0 {
		allErrs = append(allErrs, field.Invalid(authenticationCacheTTLPath, config.AuthenticationCacheTTL, fmt.Sprintf("cannot be less than zero")))
	}

	if config.AuthenticationCacheSize <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("authenticationCacheSize"), config.AuthenticationCacheSize, fmt.Sprintf("must be greater than zero")))
	}

	authorizationCacheTTLPath := fldPath.Child("authorizationCacheTTL")
	if len(config.AuthorizationCacheTTL) == 0 {
		allErrs = append(allErrs, field.Required(authorizationCacheTTLPath, ""))
	} else if ttl, err := time.ParseDuration(config.AuthorizationCacheTTL); err != nil {
		allErrs = append(allErrs, field.Invalid(authorizationCacheTTLPath, config.AuthorizationCacheTTL, fmt.Sprintf("%v", err)))
	} else if ttl < 0 {
		allErrs = append(allErrs, field.Invalid(authorizationCacheTTLPath, config.AuthorizationCacheTTL, fmt.Sprintf("cannot be less than zero")))
	}

	if config.AuthorizationCacheSize <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("authorizationCacheSize"), config.AuthorizationCacheSize, fmt.Sprintf("must be greater than zero")))
	}

	return allErrs
}

func ValidateNetworkConfig(config api.NodeNetworkConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config.MTU == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("mtu"), config.MTU, fmt.Sprintf("must be greater than zero")))
	}

	if len(config.PodTrafficNodeInterface) > 0 || len(config.PodTrafficNodeIP) > 0 {
		allErrs = append(allErrs, ValidatePodTrafficParams(config.PodTrafficNodeInterface, config.PodTrafficNodeIP, fldPath)...)
	}

	return allErrs
}

func ValidateDockerConfig(config api.DockerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	switch config.ExecHandlerName {
	case api.DockerExecHandlerNative, api.DockerExecHandlerNsenter:
		// ok
	default:
		validValues := strings.Join([]string{string(api.DockerExecHandlerNative), string(api.DockerExecHandlerNsenter)}, ", ")
		allErrs = append(allErrs, field.Invalid(fldPath.Child("execHandlerName"), config.ExecHandlerName, fmt.Sprintf("must be one of %s", validValues)))
	}

	return allErrs
}

func ValidateKubeletExtendedArguments(config api.ExtendedArguments, fldPath *field.Path) field.ErrorList {
	server, _ := kubeletoptions.NewKubeletServer()
	return ValidateExtendedArguments(config, server.AddFlags, fldPath)
}

func ValidateVolumeConfig(config api.NodeVolumeConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config.LocalQuota.PerFSGroup != nil && config.LocalQuota.PerFSGroup.Value() < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("localQuota", "perFSGroup"), config.LocalQuota.PerFSGroup,
			"must be a positive integer"))
	}
	return allErrs
}

func ValidatePodTrafficParams(nodeIface, nodeIP string, fldPath *field.Path) field.ErrorList {
	return validateNetworkInterfaceAndIP(nodeIface, nodeIP, fldPath.Child("podTrafficNodeInterface"), fldPath.Child("podTrafficNodeIP"))
}

func validateNetworkInterfaceAndIP(iface, ip string, ifaceFieldPath, ipFieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var ipv4Addrs []net.IP
	if len(iface) > 0 {
		var err error
		if ipv4Addrs, err = netutils.GetIPAddrsFromNetworkInterface(iface); err != nil {
			allErrs = append(allErrs, field.Invalid(ifaceFieldPath, iface, err.Error()))
		}
	}

	if len(ip) > 0 {
		ipObj := net.ParseIP(ip)
		if ipObj == nil {
			allErrs = append(allErrs, field.Invalid(ipFieldPath, ip, "must be a valid IP"))
		} else if ipObj.IsUnspecified() {
			allErrs = append(allErrs, field.Invalid(ipFieldPath, ip, "cannot be an unspecified IP"))
		} else if ipObj.To4() == nil {
			allErrs = append(allErrs, field.Invalid(ipFieldPath, ip, "must be IPv4 address"))
		}

		if len(iface) > 0 {
			found := false
			for _, addr := range ipv4Addrs {
				if addr.Equal(ipObj) {
					found = true
					break
				}
			}
			if !found {
				allErrs = append(allErrs, field.Invalid(ipFieldPath, ip, fmt.Sprintf("IP %q not found in network interface %q", ip, iface)))
			}
		}
	}

	return allErrs
}
