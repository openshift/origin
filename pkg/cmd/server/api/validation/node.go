package validation

import (
	"fmt"
	"strings"
	"time"

	kubeletoptions "k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/cmd/server/api"
)

func ValidateNodeConfig(config *api.NodeConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if len(config.NodeName) == 0 {
		validationResults.AddErrors(field.Required(fldPath.Child("nodeName"), ""))
	}
	if len(config.NodeIP) > 0 {
		validationResults.AddErrors(ValidateSpecifiedIP(config.NodeIP, fldPath.Child("nodeIP"))...)
	}

	servingInfoPath := fldPath.Child("servingInfo")
	validationResults.Append(ValidateServingInfo(config.ServingInfo, servingInfoPath))
	if config.ServingInfo.BindNetwork == "tcp6" {
		validationResults.AddErrors(field.Invalid(servingInfoPath.Child("bindNetwork"), config.ServingInfo.BindNetwork, "tcp6 is not a valid bindNetwork for nodes, must be tcp or tcp4"))
	}
	validationResults.AddErrors(ValidateKubeConfig(config.MasterKubeConfig, fldPath.Child("masterKubeConfig"))...)

	if len(config.DNSIP) > 0 {
		validationResults.AddErrors(ValidateSpecifiedIP(config.DNSIP, fldPath.Child("dnsIP"))...)
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
	return ValidateExtendedArguments(config, kubeletoptions.NewKubeletServer().AddFlags, fldPath)
}

func ValidateVolumeConfig(config api.NodeVolumeConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config.LocalQuota.PerFSGroup != nil && config.LocalQuota.PerFSGroup.Value() < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("localQuota", "perFSGroup"), config.LocalQuota.PerFSGroup,
			"must be a positive integer"))
	}
	return allErrs
}
