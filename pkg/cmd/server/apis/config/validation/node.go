package validation

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/validation/field"
	kubeletoptions "k8s.io/kubernetes/cmd/kubelet/app/options"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation/common"
)

func ValidateNodeConfig(config *configapi.NodeConfig, fldPath *field.Path) common.ValidationResults {
	validationResults := ValidateInClusterNodeConfig(config, fldPath)
	if bootstrap := config.KubeletArguments["bootstrap-kubeconfig"]; len(bootstrap) > 0 {
		validationResults.AddErrors(ValidateKubeConfig(bootstrap[0], fldPath.Child("kubeletArguments", "bootstrap-kubeconfig"))...)
	} else {
		validationResults.AddErrors(ValidateKubeConfig(config.MasterKubeConfig, fldPath.Child("masterKubeConfig"))...)
	}
	return validationResults
}

func ValidateInClusterNodeConfig(config *configapi.NodeConfig, fldPath *field.Path) common.ValidationResults {
	validationResults := common.ValidationResults{}

	hasBootstrapConfig := len(config.KubeletArguments["bootstrap-kubeconfig"]) > 0
	if len(config.NodeName) == 0 && !hasBootstrapConfig {
		validationResults.AddErrors(field.Required(fldPath.Child("nodeName"), ""))
	}
	if len(config.NodeIP) > 0 {
		validationResults.AddErrors(common.ValidateSpecifiedIP(config.NodeIP, fldPath.Child("nodeIP"))...)
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
		if !hasBootstrapConfig || config.DNSIP != "0.0.0.0" {
			validationResults.AddErrors(common.ValidateSpecifiedIP(config.DNSIP, fldPath.Child("dnsIP"))...)
		}
	}
	for i, nameserver := range config.DNSNameservers {
		validationResults.AddErrors(common.ValidateSpecifiedIPPort(nameserver, fldPath.Child("dnsNameservers").Index(i))...)
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

func ValidateNodeAuthConfig(config configapi.NodeAuthConfig, fldPath *field.Path) field.ErrorList {
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

func ValidateNetworkConfig(config configapi.NodeNetworkConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config.MTU == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("mtu"), config.MTU, fmt.Sprintf("must be greater than zero")))
	}
	return allErrs
}

func ValidateDockerConfig(config configapi.DockerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	switch config.ExecHandlerName {
	case configapi.DockerExecHandlerNative, configapi.DockerExecHandlerNsenter:
		// ok
	default:
		validValues := strings.Join([]string{string(configapi.DockerExecHandlerNative), string(configapi.DockerExecHandlerNsenter)}, ", ")
		allErrs = append(allErrs, field.Invalid(fldPath.Child("execHandlerName"), config.ExecHandlerName, fmt.Sprintf("must be one of %s", validValues)))
	}

	return allErrs
}

func ValidateKubeletExtendedArguments(config configapi.ExtendedArguments, fldPath *field.Path) field.ErrorList {
	server, _ := kubeletoptions.NewKubeletServer()
	return ValidateExtendedArguments(config, server.AddFlags, fldPath)
}

func ValidateVolumeConfig(config configapi.NodeVolumeConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config.LocalQuota.PerFSGroup != nil && config.LocalQuota.PerFSGroup.Value() < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("localQuota", "perFSGroup"), config.LocalQuota.PerFSGroup,
			"must be a positive integer"))
	}
	return allErrs
}
