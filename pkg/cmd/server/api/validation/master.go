package validation

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/cmd/server/api"
)

func ValidateMasterConfig(config *api.MasterConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if _, urlErrs := ValidateURL(config.MasterPublicURL, "masterPublicURL"); len(urlErrs) > 0 {
		allErrs = append(allErrs, urlErrs...)
	}

	if config.AssetConfig != nil {
		allErrs = append(allErrs, ValidateAssetConfig(config.AssetConfig).Prefix("assetConfig")...)
		colocated := config.AssetConfig.ServingInfo.BindAddress == config.ServingInfo.BindAddress
		if colocated {
			publicURL, _ := url.Parse(config.AssetConfig.PublicURL)
			if publicURL.Path == "/" {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("assetConfig.publicURL", config.AssetConfig.PublicURL, "path can not be / when colocated with master API"))
			}
		}

		if config.OAuthConfig != nil {
			if config.OAuthConfig.AssetPublicURL != config.AssetConfig.PublicURL {
				allErrs = append(allErrs,
					fielderrors.NewFieldInvalid("assetConfig.publicURL", config.AssetConfig.PublicURL, "must match oauthConfig.assetPublicURL"),
					fielderrors.NewFieldInvalid("oauthConfig.assetPublicURL", config.OAuthConfig.AssetPublicURL, "must match assetConfig.publicURL"),
				)
			}
		}

		// TODO warn when the CORS list does not include the assetConfig.publicURL host:port
		// only warn cause they could handle CORS headers themselves in a proxy
	}

	if config.DNSConfig != nil {
		allErrs = append(allErrs, ValidateHostPort(config.DNSConfig.BindAddress, "bindAddress").Prefix("dnsConfig")...)
	}

	if config.EtcdConfig != nil {
		etcdConfigErrs := ValidateEtcdConfig(config.EtcdConfig).Prefix("etcdConfig")
		allErrs = append(allErrs, etcdConfigErrs...)

		if len(etcdConfigErrs) == 0 {
			// Validate the etcdClientInfo with the internal etcdConfig
			allErrs = append(allErrs, ValidateEtcdConnectionInfo(config.EtcdClientInfo, config.EtcdConfig).Prefix("etcdClientInfo")...)
		} else {
			// Validate the etcdClientInfo by itself
			allErrs = append(allErrs, ValidateEtcdConnectionInfo(config.EtcdClientInfo, nil).Prefix("etcdClientInfo")...)
		}
	} else {
		// Validate the etcdClientInfo by itself
		allErrs = append(allErrs, ValidateEtcdConnectionInfo(config.EtcdClientInfo, nil).Prefix("etcdClientInfo")...)
	}
	allErrs = append(allErrs, ValidateEtcdStorageConfig(config.EtcdStorageConfig).Prefix("etcdStorageConfig")...)

	allErrs = append(allErrs, ValidateImageConfig(config.ImageConfig).Prefix("imageConfig")...)

	allErrs = append(allErrs, ValidateKubeletConnectionInfo(config.KubeletClientInfo).Prefix("kubeletClientInfo")...)

	if config.KubernetesMasterConfig != nil {
		allErrs = append(allErrs, ValidateKubernetesMasterConfig(config.KubernetesMasterConfig).Prefix("kubernetesMasterConfig")...)
	}
	if (config.KubernetesMasterConfig == nil) && (len(config.MasterClients.ExternalKubernetesKubeConfig) == 0) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("kubernetesMasterConfig", config.KubernetesMasterConfig, "either kubernetesMasterConfig or masterClients.externalKubernetesKubeConfig must have a value"))
	}
	if (config.KubernetesMasterConfig != nil) && (len(config.MasterClients.ExternalKubernetesKubeConfig) != 0) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("kubernetesMasterConfig", config.KubernetesMasterConfig, "kubernetesMasterConfig and masterClients.externalKubernetesKubeConfig are mutually exclusive"))
	}

	allErrs = append(allErrs, ValidateKubeConfig(config.MasterClients.DeployerKubeConfig, "deployerKubeConfig").Prefix("masterClients")...)
	allErrs = append(allErrs, ValidateKubeConfig(config.MasterClients.OpenShiftLoopbackKubeConfig, "openShiftLoopbackKubeConfig").Prefix("masterClients")...)

	if len(config.MasterClients.ExternalKubernetesKubeConfig) > 0 {
		allErrs = append(allErrs, ValidateKubeConfig(config.MasterClients.ExternalKubernetesKubeConfig, "externalKubernetesKubeConfig").Prefix("masterClients")...)
	}

	allErrs = append(allErrs, ValidatePolicyConfig(config.PolicyConfig).Prefix("policyConfig")...)
	if config.OAuthConfig != nil {
		allErrs = append(allErrs, ValidateOAuthConfig(config.OAuthConfig).Prefix("oauthConfig")...)
	}

	allErrs = append(allErrs, ValidateServingInfo(config.ServingInfo).Prefix("servingInfo")...)

	allErrs = append(allErrs, ValidateProjectRequestConfig(config.ProjectRequestConfig).Prefix("projectRequestConfig")...)

	return allErrs
}

func ValidateEtcdStorageConfig(config api.EtcdStorageConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.KubernetesStorageVersion) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("kubernetesStorageVersion"))
	}
	if len(config.OpenShiftStorageVersion) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("openShiftStorageVersion"))
	}

	return allErrs
}

func ValidateAssetConfig(config *api.AssetConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateServingInfo(config.ServingInfo).Prefix("servingInfo")...)

	if len(config.LogoutURL) > 0 {
		_, urlErrs := ValidateURL(config.LogoutURL, "logoutURL")
		if len(urlErrs) > 0 {
			allErrs = append(allErrs, urlErrs...)
		}
	}

	urlObj, urlErrs := ValidateURL(config.PublicURL, "publicURL")
	if len(urlErrs) > 0 {
		allErrs = append(allErrs, urlErrs...)
	}
	if urlObj != nil {
		if !strings.HasSuffix(urlObj.Path, "/") {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("publicURL", config.PublicURL, "must have a trailing slash in path"))
		}
	}

	if _, urlErrs := ValidateURL(config.MasterPublicURL, "masterPublicURL"); len(urlErrs) > 0 {
		allErrs = append(allErrs, urlErrs...)
	}

	return allErrs
}

func ValidateImageConfig(config api.ImageConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.Format) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("format"))
	}

	return allErrs
}

func ValidateKubeletConnectionInfo(config api.KubeletConnectionInfo) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if config.Port == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("port"))
	}

	if len(config.CA) > 0 {
		allErrs = append(allErrs, ValidateFile(config.CA, "ca")...)
	}
	allErrs = append(allErrs, ValidateCertInfo(config.ClientCert, false)...)

	return allErrs
}

func ValidateKubernetesMasterConfig(config *api.KubernetesMasterConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.MasterIP) > 0 {
		allErrs = append(allErrs, ValidateSpecifiedIP(config.MasterIP, "masterIP")...)
	}

	if len(config.ServicesSubnet) > 0 {
		if _, _, err := net.ParseCIDR(strings.TrimSpace(config.ServicesSubnet)); err != nil {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("servicesSubnet", config.ServicesSubnet, "must be a valid CIDR notation IP range (e.g. 172.30.0.0/16)"))
		}
	}

	if len(config.SchedulerConfigFile) > 0 {
		allErrs = append(allErrs, ValidateFile(config.SchedulerConfigFile, "schedulerConfigFile")...)
	}

	for i, nodeName := range config.StaticNodeNames {
		if len(nodeName) == 0 {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid(fmt.Sprintf("staticNodeName[%d]", i), nodeName, "may not be empty"))
		}
	}

	return allErrs
}

func ValidatePolicyConfig(config api.PolicyConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateFile(config.BootstrapPolicyFile, "bootstrapPolicyFile")...)
	allErrs = append(allErrs, ValidateNamespace(config.MasterAuthorizationNamespace, "masterAuthorizationNamespace")...)
	allErrs = append(allErrs, ValidateNamespace(config.OpenShiftSharedResourcesNamespace, "openShiftSharedResourcesNamespace")...)

	return allErrs
}

// ValidateProjectRequestConfig is stub for now.  no validation is required.
func ValidateProjectRequestConfig(config api.ProjectRequestConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	return allErrs
}
