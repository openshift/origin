package validation

import (
	"net"
	"os"
	"strings"

	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kvalidation "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"

	"github.com/openshift/origin/pkg/cmd/server/api"
)

func ValidateBindAddress(bindAddress string) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	if len(bindAddress) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("bindAddress"))
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
			allErrs = append(allErrs, errs.NewFieldRequired("keyFile"))
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

func ValidateKubeConfig(path string, field string) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	allErrs = append(allErrs, ValidateFile(path, field)...)
	// TODO: load and parse

	return allErrs
}

func ValidateKubernetesMasterConfig(config *api.KubernetesMasterConfig) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	if len(config.MasterIP) > 0 {
		allErrs = append(allErrs, ValidateSpecifiedIP(config.MasterIP, "masterIP")...)
	}

	if len(config.ServicesSubnet) > 0 {
		if _, _, err := net.ParseCIDR(strings.TrimSpace(config.ServicesSubnet)); err != nil {
			allErrs = append(allErrs, errs.NewFieldInvalid("servicesSubnet", config.ServicesSubnet, "must be a valid CIDR notation IP range (e.g. 172.30.17.0/24)"))
		}
	}

	return allErrs
}

func ValidateSpecifiedIP(ipString string, field string) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	ip := net.ParseIP(ipString)
	if ip == nil {
		allErrs = append(allErrs, errs.NewFieldInvalid(field, ipString, "must be a valid IP"))
	} else if ip.IsUnspecified() {
		allErrs = append(allErrs, errs.NewFieldInvalid(field, ipString, "cannot be an unspecified IP"))
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

	if config.KubernetesMasterConfig != nil {
		allErrs = append(allErrs, ValidateKubernetesMasterConfig(config.KubernetesMasterConfig).Prefix("kubernetesMasterConfig")...)
	}

	allErrs = append(allErrs, ValidatePolicyConfig(config.PolicyConfig).Prefix("policyConfig")...)

	allErrs = append(allErrs, ValidateKubeConfig(config.MasterClients.DeployerKubeConfig, "deployerKubeConfig").Prefix("masterClients")...)
	allErrs = append(allErrs, ValidateKubeConfig(config.MasterClients.OpenShiftLoopbackKubeConfig, "openShiftLoopbackKubeConfig").Prefix("masterClients")...)
	allErrs = append(allErrs, ValidateKubeConfig(config.MasterClients.KubernetesKubeConfig, "kubernetesKubeConfig").Prefix("masterClients")...)

	return allErrs
}

func ValidatePolicyConfig(config api.PolicyConfig) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	allErrs = append(allErrs, ValidateFile(config.BootstrapPolicyFile, "bootstrapPolicyFile")...)
	allErrs = append(allErrs, ValidateNamespace(config.MasterAuthorizationNamespace, "masterAuthorizationNamespace")...)
	allErrs = append(allErrs, ValidateNamespace(config.OpenShiftSharedResourcesNamespace, "openShiftSharedResourcesNamespace")...)

	return allErrs
}

func ValidateNamespace(namespace, field string) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	if len(namespace) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired(field))
	} else if ok, _ := kvalidation.ValidateNamespaceName(namespace, false); !ok {
		allErrs = append(allErrs, errs.NewFieldInvalid(field, namespace, "must be a valid namespace"))
	}

	return allErrs
}

func ValidateNodeConfig(config *api.NodeConfig) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	if len(config.NodeName) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("nodeName"))
	}

	allErrs = append(allErrs, ValidateServingInfo(config.ServingInfo).Prefix("servingInfo")...)
	allErrs = append(allErrs, ValidateKubeConfig(config.MasterKubeConfig, "masterKubeConfig")...)

	if len(config.DNSIP) > 0 {
		allErrs = append(allErrs, ValidateSpecifiedIP(config.DNSIP, "dnsIP")...)
	}

	if len(config.NetworkContainerImage) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("networkContainerImage"))
	}

	return allErrs
}

func ValidateFile(path string, field string) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}

	if len(path) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired(field))
	} else if _, err := os.Stat(path); err != nil {
		allErrs = append(allErrs, errs.NewFieldInvalid(field, path, "could not read file"))
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
