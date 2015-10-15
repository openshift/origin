package validation

import (
	"fmt"
	"strings"
	"time"

	kapp "k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/cmd/server/api"
)

func ValidateNodeConfig(config *api.NodeConfig) ValidationResults {
	validationResults := ValidationResults{}

	if len(config.NodeName) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("nodeName"))
	}
	if len(config.NodeIP) > 0 {
		validationResults.AddErrors(ValidateSpecifiedIP(config.NodeIP, "nodeIP")...)
	}

	validationResults.Append(ValidateServingInfo(config.ServingInfo).Prefix("servingInfo"))
	if config.ServingInfo.BindNetwork == "tcp6" {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("servingInfo.bindNetwork", config.ServingInfo.BindNetwork, "tcp6 is not a valid bindNetwork for nodes, must be tcp or tcp4"))
	}
	validationResults.AddErrors(ValidateKubeConfig(config.MasterKubeConfig, "masterKubeConfig")...)

	if len(config.DNSIP) > 0 {
		validationResults.AddErrors(ValidateSpecifiedIP(config.DNSIP, "dnsIP")...)
	}

	validationResults.AddErrors(ValidateImageConfig(config.ImageConfig).Prefix("imageConfig")...)

	if config.PodManifestConfig != nil {
		validationResults.AddErrors(ValidatePodManifestConfig(config.PodManifestConfig).Prefix("podManifestConfig")...)
	}

	validationResults.AddErrors(ValidateNetworkConfig(config.NetworkConfig).Prefix("networkConfig")...)

	validationResults.AddErrors(ValidateDockerConfig(config.DockerConfig).Prefix("dockerConfig")...)

	validationResults.AddErrors(ValidateNodeAuthConfig(config.AuthConfig).Prefix("authConfig")...)

	validationResults.AddErrors(ValidateKubeletExtendedArguments(config.KubeletArguments).Prefix("kubeletArguments")...)

	if _, err := time.ParseDuration(config.IPTablesSyncPeriod); err != nil {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("iptablesSyncPeriod", config.IPTablesSyncPeriod, fmt.Sprintf("unable to parse iptablesSyncPeriod: %v. Examples with correct format: '5s', '1m', '2h22m'", err)))
	}

	return validationResults
}

func ValidateNodeAuthConfig(config api.NodeAuthConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.AuthenticationCacheTTL) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("authenticationCacheTTL"))
	} else if ttl, err := time.ParseDuration(config.AuthenticationCacheTTL); err != nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("authenticationCacheTTL", config.AuthenticationCacheTTL, fmt.Sprintf("%v", err)))
	} else if ttl < 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("authenticationCacheTTL", config.AuthenticationCacheTTL, fmt.Sprintf("cannot be less than zero")))
	}

	if config.AuthenticationCacheSize <= 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("authenticationCacheSize", config.AuthenticationCacheSize, fmt.Sprintf("must be greater than zero")))
	}

	if len(config.AuthorizationCacheTTL) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("authorizationCacheTTL"))
	} else if ttl, err := time.ParseDuration(config.AuthorizationCacheTTL); err != nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("authorizationCacheTTL", config.AuthorizationCacheTTL, fmt.Sprintf("%v", err)))
	} else if ttl < 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("authorizationCacheTTL", config.AuthorizationCacheTTL, fmt.Sprintf("cannot be less than zero")))
	}

	if config.AuthorizationCacheSize <= 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("authorizationCacheSize", config.AuthorizationCacheSize, fmt.Sprintf("must be greater than zero")))
	}

	return allErrs
}

func ValidateNetworkConfig(config api.NodeNetworkConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.NetworkPluginName) > 0 {
		if config.MTU == 0 {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("mtu", config.MTU, fmt.Sprintf("must be greater than zero")))
		}
	}
	return allErrs
}

func ValidateDockerConfig(config api.DockerConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	switch config.ExecHandlerName {
	case api.DockerExecHandlerNative, api.DockerExecHandlerNsenter:
		// ok
	default:
		validValues := strings.Join([]string{string(api.DockerExecHandlerNative), string(api.DockerExecHandlerNsenter)}, ", ")
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("execHandlerName", config.ExecHandlerName, fmt.Sprintf("must be one of %s", validValues)))
	}

	return allErrs
}

func ValidateKubeletExtendedArguments(config api.ExtendedArguments) fielderrors.ValidationErrorList {
	return ValidateExtendedArguments(config, kapp.NewKubeletServer().AddFlags)
}
