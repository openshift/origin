package validation

import (
	"net"
	"os"

	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kvalidation "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"

	"github.com/openshift/origin/pkg/cmd/server/api"
)

func ValidateBindAddress(bindAddress string) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	if len(bindAddress) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("bindAddress", bindAddress))
	} else if _, _, err := net.SplitHostPort(bindAddress); err != nil {
		allErrs = append(allErrs, errs.NewFieldInvalid("bindAddress", bindAddress, "must be a host:port"))
	}

	return allErrs
}

func ValidateServingInfo(info api.ServingInfo) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	allErrs = append(allErrs, ValidateBindAddress(info.BindAddress)...)

	if len(info.ServerCert.CertFile) > 0 {
		if _, err := os.Stat(info.ServerCert.CertFile); err != nil {
			allErrs = append(allErrs, errs.NewFieldInvalid("certFile", info.ServerCert.CertFile, "could not read file"))
		}

		if len(info.ServerCert.KeyFile) == 0 {
			allErrs = append(allErrs, errs.NewFieldRequired("keyFile", info.ServerCert.KeyFile))
		} else if _, err := os.Stat(info.ServerCert.KeyFile); err != nil {
			allErrs = append(allErrs, errs.NewFieldInvalid("keyFile", info.ServerCert.KeyFile, "could not read file"))
		}

		if len(info.ClientCA) > 0 {
			if _, err := os.Stat(info.ClientCA); err != nil {
				allErrs = append(allErrs, errs.NewFieldInvalid("clientCA", info.ClientCA, "could not read file"))
			}
		}
	} else {
		if len(info.ServerCert.KeyFile) > 0 {
			allErrs = append(allErrs, errs.NewFieldInvalid("keyFile", info.ServerCert.KeyFile, "cannot specify a keyFile without a certFile"))
		}

		if len(info.ClientCA) > 0 {
			allErrs = append(allErrs, errs.NewFieldInvalid("clientCA", info.ClientCA, "cannot specify a clientCA without a certFile"))
		}
	}

	return allErrs
}

func ValidateMasterConfig(config *api.MasterConfig) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	allErrs = append(allErrs, ValidateServingInfo(config.ServingInfo).Prefix("servingInfo")...)

	if config.AssetConfig != nil {
		allErrs = append(allErrs, ValidateServingInfo(config.AssetConfig.ServingInfo).Prefix("assetConfig.servingInfo")...)
	}

	if config.DNSConfig != nil {
		allErrs = append(allErrs, ValidateBindAddress(config.DNSConfig.BindAddress).Prefix("dnsConfig")...)
	}

	if len(config.MasterAuthorizationNamespace) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("masterAuthorizationNamespace", config.MasterAuthorizationNamespace))
	} else if ok, _ := kvalidation.ValidateNamespaceName(config.MasterAuthorizationNamespace, false); !ok {
		allErrs = append(allErrs, errs.NewFieldInvalid("masterAuthorizationNamespace", config.MasterAuthorizationNamespace, "must be a valid namespace"))
	}

	return allErrs
}

func ValidateNodeConfig(config *api.NodeConfig) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	if len(config.NodeName) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("nodeName", config.NodeName))
	}

	allErrs = append(allErrs, ValidateServingInfo(config.ServingInfo).Prefix("servingInfo")...)

	if len(config.MasterKubeConfig) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("masterKubeConfig", config.MasterKubeConfig))
	} else if _, err := os.Stat(config.MasterKubeConfig); err != nil {
		allErrs = append(allErrs, errs.NewFieldInvalid("masterKubeConfig", config.MasterKubeConfig, "could not read file"))
	}

	if len(config.NetworkContainerImage) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("networkContainerImage", config.NetworkContainerImage))
	}

	return allErrs
}

func ValidateAllInOneConfig(master *api.MasterConfig, node *api.NodeConfig) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	allErrs = append(allErrs, ValidateMasterConfig(master).Prefix("masterConfig")...)

	allErrs = append(allErrs, ValidateNodeConfig(node).Prefix("nodeConfig")...)

	// Validation between the configs

	return allErrs
}
