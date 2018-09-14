package node

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/validation/field"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation/common"
)

func ValidateInClusterNetworkNodeConfig(config *configapi.NodeConfig, fldPath *field.Path) common.ValidationResults {
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
	validationResults.Append(common.ValidateServingInfo(config.ServingInfo, !hasCertDir, servingInfoPath))
	if config.ServingInfo.BindNetwork == "tcp6" {
		validationResults.AddErrors(field.Invalid(servingInfoPath.Child("bindNetwork"), config.ServingInfo.BindNetwork, "tcp6 is not a valid bindNetwork for nodes, must be tcp or tcp4"))
	}

	validationResults.AddErrors(ValidateNetworkConfig(config.NetworkConfig, fldPath.Child("networkConfig"))...)

	validationResults.AddErrors(ValidateDockerConfig(config.DockerConfig, fldPath.Child("dockerConfig"))...)

	if _, err := time.ParseDuration(config.IPTablesSyncPeriod); err != nil {
		validationResults.AddErrors(field.Invalid(fldPath.Child("iptablesSyncPeriod"), config.IPTablesSyncPeriod, fmt.Sprintf("unable to parse iptablesSyncPeriod: %v. Examples with correct format: '5s', '1m', '2h22m'", err)))
	}

	return validationResults
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
