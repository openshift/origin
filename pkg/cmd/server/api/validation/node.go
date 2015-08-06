package validation

import (
	"fmt"
	"strings"

	kapp "k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/cmd/server/api"
)

func ValidateNodeConfig(config *api.NodeConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.NodeName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("nodeName"))
	}

	allErrs = append(allErrs, ValidateServingInfo(config.ServingInfo).Prefix("servingInfo")...)
	if config.ServingInfo.BindNetwork == "tcp6" {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("servingInfo.bindNetwork", config.ServingInfo.BindNetwork, "tcp6 is not a valid bindNetwork for nodes, must be tcp or tcp4"))
	}
	allErrs = append(allErrs, ValidateKubeConfig(config.MasterKubeConfig, "masterKubeConfig")...)

	if len(config.DNSIP) > 0 {
		allErrs = append(allErrs, ValidateSpecifiedIP(config.DNSIP, "dnsIP")...)
	}

	allErrs = append(allErrs, ValidateImageConfig(config.ImageConfig).Prefix("imageConfig")...)

	if config.PodManifestConfig != nil {
		allErrs = append(allErrs, ValidatePodManifestConfig(config.PodManifestConfig).Prefix("podManifestConfig")...)
	}

	allErrs = append(allErrs, ValidateNetworkConfig(config.NetworkConfig).Prefix("networkConfig")...)

	allErrs = append(allErrs, ValidateDockerConfig(config.DockerConfig).Prefix("dockerConfig")...)

	allErrs = append(allErrs, ValidateKubeletExtendedArguments(config.KubeletArguments).Prefix("kubeletArguments")...)

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
