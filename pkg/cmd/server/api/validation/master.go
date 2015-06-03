package validation

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	kvalidation "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/serviceaccount"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/util/labelselector"
)

type ValidationResults struct {
	Warnings fielderrors.ValidationErrorList
	Errors   fielderrors.ValidationErrorList
}

func (r *ValidationResults) Append(additionalResults ValidationResults) {
	r.AddErrors(additionalResults.Errors...)
	r.AddWarnings(additionalResults.Warnings...)
}

func (r *ValidationResults) AddErrors(errors ...error) {
	if len(errors) == 0 {
		return
	}
	r.Errors = append(r.Errors, errors...)
}

func (r *ValidationResults) AddWarnings(warnings ...error) {
	if len(warnings) == 0 {
		return
	}
	r.Warnings = append(r.Warnings, warnings...)
}

func (r ValidationResults) Prefix(prefix string) ValidationResults {
	r.Warnings = r.Warnings.Prefix(prefix)
	r.Errors = r.Errors.Prefix(prefix)
	return r
}

func ValidateMasterConfig(config *api.MasterConfig) ValidationResults {
	validationResults := ValidationResults{}

	if _, urlErrs := ValidateURL(config.MasterPublicURL, "masterPublicURL"); len(urlErrs) > 0 {
		validationResults.AddErrors(urlErrs...)
	}

	if config.AssetConfig != nil {
		validationResults.AddErrors(ValidateAssetConfig(config.AssetConfig).Prefix("assetConfig")...)
		colocated := config.AssetConfig.ServingInfo.BindAddress == config.ServingInfo.BindAddress
		if colocated {
			publicURL, _ := url.Parse(config.AssetConfig.PublicURL)
			if publicURL.Path == "/" {
				validationResults.AddErrors(fielderrors.NewFieldInvalid("assetConfig.publicURL", config.AssetConfig.PublicURL, "path can not be / when colocated with master API"))
			}
		}

		if config.OAuthConfig != nil {
			if config.OAuthConfig.AssetPublicURL != config.AssetConfig.PublicURL {
				validationResults.AddErrors(
					fielderrors.NewFieldInvalid("assetConfig.publicURL", config.AssetConfig.PublicURL, "must match oauthConfig.assetPublicURL"),
					fielderrors.NewFieldInvalid("oauthConfig.assetPublicURL", config.OAuthConfig.AssetPublicURL, "must match assetConfig.publicURL"),
				)
			}
		}

		// TODO warn when the CORS list does not include the assetConfig.publicURL host:port
		// only warn cause they could handle CORS headers themselves in a proxy
	}

	if config.DNSConfig != nil {
		validationResults.AddErrors(ValidateHostPort(config.DNSConfig.BindAddress, "bindAddress").Prefix("dnsConfig")...)
	}

	if config.EtcdConfig != nil {
		etcdConfigErrs := ValidateEtcdConfig(config.EtcdConfig).Prefix("etcdConfig")
		validationResults.AddErrors(etcdConfigErrs...)

		if len(etcdConfigErrs) == 0 {
			// Validate the etcdClientInfo with the internal etcdConfig
			validationResults.AddErrors(ValidateEtcdConnectionInfo(config.EtcdClientInfo, config.EtcdConfig).Prefix("etcdClientInfo")...)
		} else {
			// Validate the etcdClientInfo by itself
			validationResults.AddErrors(ValidateEtcdConnectionInfo(config.EtcdClientInfo, nil).Prefix("etcdClientInfo")...)
		}
	} else {
		// Validate the etcdClientInfo by itself
		validationResults.AddErrors(ValidateEtcdConnectionInfo(config.EtcdClientInfo, nil).Prefix("etcdClientInfo")...)
	}
	validationResults.AddErrors(ValidateEtcdStorageConfig(config.EtcdStorageConfig).Prefix("etcdStorageConfig")...)

	validationResults.AddErrors(ValidateImageConfig(config.ImageConfig).Prefix("imageConfig")...)

	validationResults.AddErrors(ValidateKubeletConnectionInfo(config.KubeletClientInfo).Prefix("kubeletClientInfo")...)

	builtInKubernetes := config.KubernetesMasterConfig != nil
	if config.KubernetesMasterConfig != nil {
		validationResults.Append(ValidateKubernetesMasterConfig(config.KubernetesMasterConfig).Prefix("kubernetesMasterConfig"))
	}
	if (config.KubernetesMasterConfig == nil) && (len(config.MasterClients.ExternalKubernetesKubeConfig) == 0) {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("kubernetesMasterConfig", config.KubernetesMasterConfig, "either kubernetesMasterConfig or masterClients.externalKubernetesKubeConfig must have a value"))
	}
	if (config.KubernetesMasterConfig != nil) && (len(config.MasterClients.ExternalKubernetesKubeConfig) != 0) {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("kubernetesMasterConfig", config.KubernetesMasterConfig, "kubernetesMasterConfig and masterClients.externalKubernetesKubeConfig are mutually exclusive"))
	}

	validationResults.AddErrors(ValidateKubeConfig(config.MasterClients.DeployerKubeConfig, "deployerKubeConfig").Prefix("masterClients")...)
	validationResults.AddErrors(ValidateKubeConfig(config.MasterClients.OpenShiftLoopbackKubeConfig, "openShiftLoopbackKubeConfig").Prefix("masterClients")...)

	if len(config.MasterClients.ExternalKubernetesKubeConfig) > 0 {
		validationResults.AddErrors(ValidateKubeConfig(config.MasterClients.ExternalKubernetesKubeConfig, "externalKubernetesKubeConfig").Prefix("masterClients")...)
	}

	validationResults.AddErrors(ValidatePolicyConfig(config.PolicyConfig).Prefix("policyConfig")...)
	if config.OAuthConfig != nil {
		validationResults.AddErrors(ValidateOAuthConfig(config.OAuthConfig).Prefix("oauthConfig")...)
	}

	validationResults.AddErrors(ValidateServiceAccountConfig(config.ServiceAccountConfig, builtInKubernetes).Prefix("serviceAccountConfig")...)

	validationResults.AddErrors(ValidateServingInfo(config.ServingInfo).Prefix("servingInfo")...)

	validationResults.AddErrors(ValidateProjectConfig(config.ProjectConfig).Prefix("projectConfig")...)

	validationResults.Append(ValidateAPILevels(config.APILevels, api.KnownOpenShiftAPILevels, api.DeadOpenShiftAPILevels, "apiLevels"))

	return validationResults
}

func ValidateAPILevels(apiLevels []string, knownAPILevels, deadAPILevels []string, name string) ValidationResults {
	validationResults := ValidationResults{}

	if len(apiLevels) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired(name))
	}

	deadLevels := util.NewStringSet(deadAPILevels...)
	knownLevels := util.NewStringSet(knownAPILevels...)
	for i, apiLevel := range apiLevels {
		if deadLevels.Has(apiLevel) {
			validationResults.AddWarnings(fielderrors.NewFieldInvalid(fmt.Sprintf(name+"[%d]", i), apiLevel, "unsupported level"))
		}
		if !knownLevels.Has(apiLevel) {
			validationResults.AddWarnings(fielderrors.NewFieldInvalid(fmt.Sprintf(name+"[%d]", i), apiLevel, "unknown level"))
		}
	}

	return validationResults
}

func ValidateEtcdStorageConfig(config api.EtcdStorageConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.KubernetesStorageVersion) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("kubernetesStorageVersion"))
	}
	if len(config.OpenShiftStorageVersion) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("openShiftStorageVersion"))
	}

	if strings.ContainsRune(config.KubernetesStoragePrefix, '%') {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("kubernetesStoragePrefix", config.KubernetesStoragePrefix, "the '%' character may not be used in etcd path prefixes"))
	}
	if strings.ContainsRune(config.OpenShiftStoragePrefix, '%') {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("openShiftStoragePrefix", config.OpenShiftStoragePrefix, "the '%' character may not be used in etcd path prefixes"))
	}

	return allErrs
}

func ValidateServiceAccountConfig(config api.ServiceAccountConfig, builtInKubernetes bool) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	managedNames := util.NewStringSet(config.ManagedNames...)
	if !managedNames.Has(bootstrappolicy.BuilderServiceAccountName) {
		// TODO: warn that default builder service account won't be auto-created
	}
	if builtInKubernetes && !managedNames.Has(bootstrappolicy.DefaultServiceAccountName) {
		// TODO: warn that default service account won't be auto-created
	}

	for i, name := range config.ManagedNames {
		if ok, msg := kvalidation.ValidateServiceAccountName(name, false); !ok {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid(fmt.Sprintf("", i), name, msg))
		}
	}

	if len(config.PrivateKeyFile) > 0 {
		if fileErrs := ValidateFile(config.PrivateKeyFile, "privateKeyFile"); len(fileErrs) > 0 {
			allErrs = append(allErrs, fileErrs...)
		} else if privateKey, err := serviceaccount.ReadPrivateKey(config.PrivateKeyFile); err != nil {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("privateKeyFile", config.PrivateKeyFile, err.Error()))
		} else if err := privateKey.Validate(); err != nil {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("privateKeyFile", config.PrivateKeyFile, err.Error()))
		}
	} else if builtInKubernetes {
		// TODO: warn that no service account tokens will be generated
	}

	if len(config.PublicKeyFiles) == 0 {
		// TODO: warn that no service accounts will be able to authenticate
	}
	for i, publicKeyFile := range config.PublicKeyFiles {
		if fileErrs := ValidateFile(publicKeyFile, fmt.Sprintf("publicKeyFiles[%d]", i)); len(fileErrs) > 0 {
			allErrs = append(allErrs, fileErrs...)
		} else if _, err := serviceaccount.ReadPublicKey(publicKeyFile); err != nil {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid(fmt.Sprintf("publicKeyFiles[%d]", i), publicKeyFile, err.Error()))
		}
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

func ValidateKubernetesMasterConfig(config *api.KubernetesMasterConfig) ValidationResults {
	validationResults := ValidationResults{}

	if len(config.MasterIP) > 0 {
		validationResults.AddErrors(ValidateSpecifiedIP(config.MasterIP, "masterIP")...)
	}

	if config.MasterCount < 1 {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("masterCount", config.MasterCount, "must be a positive integer"))
	}

	if len(config.ServicesSubnet) > 0 {
		if _, _, err := net.ParseCIDR(strings.TrimSpace(config.ServicesSubnet)); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("servicesSubnet", config.ServicesSubnet, "must be a valid CIDR notation IP range (e.g. 172.30.0.0/16)"))
		}
	}

	if len(config.SchedulerConfigFile) > 0 {
		validationResults.AddErrors(ValidateFile(config.SchedulerConfigFile, "schedulerConfigFile")...)
	}

	for i, nodeName := range config.StaticNodeNames {
		if len(nodeName) == 0 {
			validationResults.AddErrors(fielderrors.NewFieldInvalid(fmt.Sprintf("staticNodeName[%d]", i), nodeName, "may not be empty"))
		}
	}

	if len(config.PodEvictionTimeout) > 0 {
		if _, err := time.ParseDuration(config.PodEvictionTimeout); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("podEvictionTimeout", config.PodEvictionTimeout, "must be a valid time duration string (e.g. '300ms' or '2m30s'). Valid time units are 'ns', 'us', 'ms', 's', 'm', 'h'"))
		}
	}

	validationResults.Append(ValidateAPILevels(config.APILevels, api.KnownKubernetesAPILevels, api.DeadKubernetesAPILevels, "apiLevels"))

	return validationResults
}

func ValidatePolicyConfig(config api.PolicyConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateFile(config.BootstrapPolicyFile, "bootstrapPolicyFile")...)
	allErrs = append(allErrs, ValidateNamespace(config.OpenShiftSharedResourcesNamespace, "openShiftSharedResourcesNamespace")...)

	return allErrs
}

func ValidateProjectConfig(config api.ProjectConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if _, _, err := api.ParseNamespaceAndName(config.ProjectRequestTemplate); err != nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("projectRequestTemplate", config.ProjectRequestTemplate, "must be in the form: namespace/templateName"))
	}

	if len(config.DefaultNodeSelector) > 0 {
		_, err := labelselector.Parse(config.DefaultNodeSelector)
		if err != nil {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("defaultNodeSelector", config.DefaultNodeSelector, "must be a valid label selector"))
		}
	}

	return allErrs
}
