package validation

import (
	"fmt"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"

	kapp "k8s.io/kubernetes/cmd/kube-apiserver/app"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kvalidation "k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/fielderrors"
	"k8s.io/kubernetes/pkg/util/sets"
	kuval "k8s.io/kubernetes/pkg/util/validation"

	"github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/security/mcs"
	"github.com/openshift/origin/pkg/security/uid"
	"github.com/openshift/origin/pkg/util/labelselector"
)

// TODO: this should just be two return arrays, no need to be clever
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

	switch {
	case config.ControllerLeaseTTL > 300,
		config.ControllerLeaseTTL < -1,
		config.ControllerLeaseTTL > 0 && config.ControllerLeaseTTL < 10:
		validationResults.AddErrors(fielderrors.NewFieldInvalid("controllerLeaseTTL", config.ControllerLeaseTTL, "TTL must be -1 (disabled), 0 (default), or between 10 and 300 seconds"))
	}

	validationResults.AddErrors(ValidateDisabledFeatures(config.DisabledFeatures, "disabledFeatures")...)

	if config.AssetConfig != nil {
		validationResults.Append(ValidateAssetConfig(config.AssetConfig).Prefix("assetConfig"))
		colocated := config.AssetConfig.ServingInfo.BindAddress == config.ServingInfo.BindAddress
		if colocated {
			publicURL, _ := url.Parse(config.AssetConfig.PublicURL)
			if publicURL.Path == "/" {
				validationResults.AddErrors(fielderrors.NewFieldInvalid("assetConfig.publicURL", config.AssetConfig.PublicURL, "path can not be / when colocated with master API"))
			}

			// Warn if they have customized the asset certificates in ways that will be ignored
			if !reflect.DeepEqual(config.AssetConfig.ServingInfo.ServerCert, config.ServingInfo.ServerCert) ||
				!reflect.DeepEqual(config.AssetConfig.ServingInfo.NamedCertificates, config.ServingInfo.NamedCertificates) {
				validationResults.AddWarnings(fielderrors.NewFieldInvalid("assetConfig.servingInfo", "<not displayed>", "changes to assetConfig certificate configuration are not used when colocated with master API"))
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
		switch config.DNSConfig.BindNetwork {
		case "tcp", "tcp4", "tcp6":
		default:
			validationResults.AddErrors(fielderrors.NewFieldInvalid("dnsConfig.bindNetwork", config.DNSConfig.BindNetwork, "must be 'tcp', 'tcp4', or 'tcp6'"))
		}
	}

	if config.EtcdConfig != nil {
		etcdConfigErrs := ValidateEtcdConfig(config.EtcdConfig).Prefix("etcdConfig")
		validationResults.Append(etcdConfigErrs)

		if len(etcdConfigErrs.Errors) == 0 {
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

	if len(config.NetworkConfig.ServiceNetworkCIDR) > 0 {
		if _, _, err := net.ParseCIDR(strings.TrimSpace(config.NetworkConfig.ServiceNetworkCIDR)); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("networkConfig.serviceNetworkCIDR", config.NetworkConfig.ServiceNetworkCIDR, "must be a valid CIDR notation IP range (e.g. 172.30.0.0/16)"))
		} else if config.KubernetesMasterConfig != nil && len(config.KubernetesMasterConfig.ServicesSubnet) > 0 && config.KubernetesMasterConfig.ServicesSubnet != config.NetworkConfig.ServiceNetworkCIDR {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("networkConfig.serviceNetworkCIDR", config.NetworkConfig.ServiceNetworkCIDR, fmt.Sprintf("must match kubernetesMasterConfig.servicesSubnet value of %q", config.KubernetesMasterConfig.ServicesSubnet)))
		}
	}

	validationResults.AddErrors(ValidateKubeConfig(config.MasterClients.OpenShiftLoopbackKubeConfig, "openShiftLoopbackKubeConfig").Prefix("masterClients")...)

	if len(config.MasterClients.ExternalKubernetesKubeConfig) > 0 {
		validationResults.AddErrors(ValidateKubeConfig(config.MasterClients.ExternalKubernetesKubeConfig, "externalKubernetesKubeConfig").Prefix("masterClients")...)
	}

	validationResults.AddErrors(ValidatePolicyConfig(config.PolicyConfig).Prefix("policyConfig")...)
	if config.OAuthConfig != nil {
		validationResults.Append(ValidateOAuthConfig(config.OAuthConfig).Prefix("oauthConfig"))
	}

	validationResults.Append(ValidateServiceAccountConfig(config.ServiceAccountConfig, builtInKubernetes).Prefix("serviceAccountConfig"))

	validationResults.Append(ValidateHTTPServingInfo(config.ServingInfo).Prefix("servingInfo"))

	validationResults.Append(ValidateProjectConfig(config.ProjectConfig).Prefix("projectConfig"))

	validationResults.AddErrors(ValidateRoutingConfig(config.RoutingConfig).Prefix("routingConfig")...)

	validationResults.Append(ValidateAPILevels(config.APILevels, api.KnownOpenShiftAPILevels, api.DeadOpenShiftAPILevels, "apiLevels"))

	return validationResults
}

func ValidateAPILevels(apiLevels []string, knownAPILevels, deadAPILevels []string, name string) ValidationResults {
	validationResults := ValidationResults{}

	if len(apiLevels) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired(name))
	}

	deadLevels := sets.NewString(deadAPILevels...)
	knownLevels := sets.NewString(knownAPILevels...)
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

	allErrs = append(allErrs, ValidateStorageVersionLevel(
		config.KubernetesStorageVersion,
		api.KnownKubernetesStorageVersionLevels,
		api.DeadKubernetesStorageVersionLevels,
		"kubernetesStorageVersion")...)
	allErrs = append(allErrs, ValidateStorageVersionLevel(
		config.OpenShiftStorageVersion,
		api.KnownOpenShiftStorageVersionLevels,
		api.DeadOpenShiftStorageVersionLevels,
		"openShiftStorageVersion")...)

	if strings.ContainsRune(config.KubernetesStoragePrefix, '%') {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("kubernetesStoragePrefix", config.KubernetesStoragePrefix, "the '%' character may not be used in etcd path prefixes"))
	}
	if strings.ContainsRune(config.OpenShiftStoragePrefix, '%') {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("openShiftStoragePrefix", config.OpenShiftStoragePrefix, "the '%' character may not be used in etcd path prefixes"))
	}

	return allErrs
}

func ValidateStorageVersionLevel(level string, knownAPILevels, deadAPILevels []string, name string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(level) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired(name))
		return allErrs
	}
	supportedLevels := sets.NewString(knownAPILevels...)
	supportedLevels.Delete(deadAPILevels...)
	if !supportedLevels.Has(level) {
		allErrs = append(allErrs, fielderrors.NewFieldValueNotSupported(name, level, supportedLevels.List()))
	}

	return allErrs
}

func ValidateServiceAccountConfig(config api.ServiceAccountConfig, builtInKubernetes bool) ValidationResults {
	validationResults := ValidationResults{}

	managedNames := sets.NewString(config.ManagedNames...)
	if !managedNames.Has(bootstrappolicy.BuilderServiceAccountName) {
		validationResults.AddWarnings(fielderrors.NewFieldInvalid("managedNames", "", fmt.Sprintf("missing %q, which will require manual creation in each namespace before builds can run", bootstrappolicy.BuilderServiceAccountName)))
	}
	if !managedNames.Has(bootstrappolicy.DeployerServiceAccountName) {
		validationResults.AddWarnings(fielderrors.NewFieldInvalid("managedNames", "", fmt.Sprintf("missing %q, which will require manual creation in each namespace before deployments can run", bootstrappolicy.DeployerServiceAccountName)))
	}
	if builtInKubernetes && !managedNames.Has(bootstrappolicy.DefaultServiceAccountName) {
		validationResults.AddWarnings(fielderrors.NewFieldInvalid("managedNames", "", fmt.Sprintf("missing %q, which will prevent creation of pods that do not specify a valid service account", bootstrappolicy.DefaultServiceAccountName)))
	}

	for i, name := range config.ManagedNames {
		if ok, msg := kvalidation.ValidateServiceAccountName(name, false); !ok {
			validationResults.AddErrors(fielderrors.NewFieldInvalid(fmt.Sprintf("managedNames[%d]", i), name, msg))
		}
	}

	if len(config.PrivateKeyFile) > 0 {
		if fileErrs := ValidateFile(config.PrivateKeyFile, "privateKeyFile"); len(fileErrs) > 0 {
			validationResults.AddErrors(fileErrs...)
		} else if privateKey, err := serviceaccount.ReadPrivateKey(config.PrivateKeyFile); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("privateKeyFile", config.PrivateKeyFile, err.Error()))
		} else if err := privateKey.Validate(); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("privateKeyFile", config.PrivateKeyFile, err.Error()))
		}
	} else if builtInKubernetes {
		validationResults.AddWarnings(fielderrors.NewFieldInvalid("privateKeyFile", "", "no service account tokens will be generated, which could prevent builds and deployments from working"))
	}

	if len(config.PublicKeyFiles) == 0 {
		validationResults.AddWarnings(fielderrors.NewFieldInvalid("publicKeyFiles", "", "no service account tokens will be accepted by the API, which will prevent builds and deployments from working"))
	}
	for i, publicKeyFile := range config.PublicKeyFiles {
		if fileErrs := ValidateFile(publicKeyFile, fmt.Sprintf("publicKeyFiles[%d]", i)); len(fileErrs) > 0 {
			validationResults.AddErrors(fileErrs...)
		} else if _, err := serviceaccount.ReadPublicKey(publicKeyFile); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid(fmt.Sprintf("publicKeyFiles[%d]", i), publicKeyFile, err.Error()))
		}
	}

	if len(config.MasterCA) > 0 {
		validationResults.AddErrors(ValidateFile(config.MasterCA, "masterCA")...)
	} else if builtInKubernetes {
		validationResults.AddWarnings(fielderrors.NewFieldInvalid("masterCA", "", "master CA information will not be automatically injected into pods, which will prevent verification of the API server from inside a pod"))
	}

	return validationResults
}

func ValidateAssetConfig(config *api.AssetConfig) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.Append(ValidateHTTPServingInfo(config.ServingInfo).Prefix("servingInfo"))

	if len(config.LogoutURL) > 0 {
		_, urlErrs := ValidateURL(config.LogoutURL, "logoutURL")
		if len(urlErrs) > 0 {
			validationResults.AddErrors(urlErrs...)
		}
	}

	urlObj, urlErrs := ValidateURL(config.PublicURL, "publicURL")
	if len(urlErrs) > 0 {
		validationResults.AddErrors(urlErrs...)
	}
	if urlObj != nil {
		if !strings.HasSuffix(urlObj.Path, "/") {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("publicURL", config.PublicURL, "must have a trailing slash in path"))
		}
	}

	if _, urlErrs := ValidateURL(config.MasterPublicURL, "masterPublicURL"); len(urlErrs) > 0 {
		validationResults.AddErrors(urlErrs...)
	}

	if len(config.LoggingPublicURL) > 0 {
		if _, loggingURLErrs := ValidateSecureURL(config.LoggingPublicURL, "loggingPublicURL"); len(loggingURLErrs) > 0 {
			validationResults.AddErrors(loggingURLErrs...)
		}
	}

	if len(config.MetricsPublicURL) > 0 {
		if _, metricsURLErrs := ValidateSecureURL(config.MetricsPublicURL, "metricsPublicURL"); len(metricsURLErrs) > 0 {
			validationResults.AddErrors(metricsURLErrs...)
		}
	}

	for i, scriptFile := range config.ExtensionScripts {
		validationResults.AddErrors(ValidateFile(scriptFile, fmt.Sprintf("extensionScripts[%d]", i))...)
	}

	for i, stylesheetFile := range config.ExtensionStylesheets {
		validationResults.AddErrors(ValidateFile(stylesheetFile, fmt.Sprintf("extensionStylesheets[%d]", i))...)
	}

	nameTaken := map[string]bool{}
	for i, extConfig := range config.Extensions {
		extConfigErrors := ValidateAssetExtensionsConfig(extConfig).Prefix(fmt.Sprintf("extensions[%d]", i))
		validationResults.AddErrors(extConfigErrors...)
		if nameTaken[extConfig.Name] {
			dupError := fielderrors.NewFieldInvalid(fmt.Sprintf("extensions[%d].name", i), extConfig.Name, "duplicate extension name")
			validationResults.AddErrors(dupError)
		} else {
			nameTaken[extConfig.Name] = true
		}
	}

	return validationResults
}

var extNameExp = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func ValidateAssetExtensionsConfig(extConfig api.AssetExtensionsConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateDir(extConfig.SourceDirectory, "sourceDirectory")...)

	if len(extConfig.Name) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("name"))
	} else if !extNameExp.MatchString(extConfig.Name) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("name", extConfig.Name, fmt.Sprintf("does not match %v", extNameExp)))
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

	if config.MasterCount == 0 || config.MasterCount < -1 {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("masterCount", config.MasterCount, "must be a positive integer or -1"))
	}

	validationResults.AddErrors(ValidateCertInfo(config.ProxyClientInfo, false).Prefix("proxyClientInfo")...)
	if len(config.ProxyClientInfo.CertFile) == 0 && len(config.ProxyClientInfo.KeyFile) == 0 {
		validationResults.AddWarnings(fielderrors.NewFieldInvalid("proxyClientInfo", "", "if no client certificate is specified, TLS pods and services cannot validate requests came from the proxy"))
	}

	if len(config.ServicesSubnet) > 0 {
		if _, _, err := net.ParseCIDR(strings.TrimSpace(config.ServicesSubnet)); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("servicesSubnet", config.ServicesSubnet, "must be a valid CIDR notation IP range (e.g. 172.30.0.0/16)"))
		}
	}

	if len(config.ServicesNodePortRange) > 0 {
		if _, err := util.ParsePortRange(strings.TrimSpace(config.ServicesNodePortRange)); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("servicesNodePortRange", config.ServicesNodePortRange, "must be a valid port range (e.g. 30000-32000)"))
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

	for group, versions := range config.DisabledAPIGroupVersions {
		name := "disabledAPIGroupVersions[" + group + "]"
		if !api.KnownKubeAPIGroups.Has(group) {
			validationResults.AddWarnings(fielderrors.NewFieldValueNotSupported(name, group, api.KnownKubeAPIGroups.List()))
			continue
		}

		allowedVersions := sets.NewString(api.KubeAPIGroupsToAllowedVersions[group]...)
		for i, version := range versions {
			if version == "*" {
				continue
			}

			if !allowedVersions.Has(version) {
				validationResults.AddWarnings(fielderrors.NewFieldValueNotSupported(fmt.Sprintf("%s[%d]", name, i), version, allowedVersions.List()))
			}
		}
	}

	validationResults.AddErrors(ValidateAPIServerExtendedArguments(config.APIServerArguments).Prefix("apiServerArguments")...)
	validationResults.AddErrors(ValidateControllerExtendedArguments(config.ControllerArguments).Prefix("controllerArguments")...)

	return validationResults
}

func ValidatePolicyConfig(config api.PolicyConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateFile(config.BootstrapPolicyFile, "bootstrapPolicyFile")...)
	allErrs = append(allErrs, ValidateNamespace(config.OpenShiftSharedResourcesNamespace, "openShiftSharedResourcesNamespace")...)
	allErrs = append(allErrs, ValidateNamespace(config.OpenShiftInfrastructureNamespace, "openShiftInfrastructureNamespace")...)

	return allErrs
}

func ValidateProjectConfig(config api.ProjectConfig) ValidationResults {
	validationResults := ValidationResults{}

	if _, _, err := api.ParseNamespaceAndName(config.ProjectRequestTemplate); err != nil {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("projectRequestTemplate", config.ProjectRequestTemplate, "must be in the form: namespace/templateName"))
	}

	if len(config.DefaultNodeSelector) > 0 {
		_, err := labelselector.Parse(config.DefaultNodeSelector)
		if err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("defaultNodeSelector", config.DefaultNodeSelector, "must be a valid label selector"))
		}
	}

	if alloc := config.SecurityAllocator; alloc != nil {
		if _, err := uid.ParseRange(alloc.UIDAllocatorRange); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("securityAllocator.uidAllocatorRange", alloc.UIDAllocatorRange, err.Error()))
		}
		if _, err := mcs.ParseRange(alloc.MCSAllocatorRange); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("securityAllocator.mcsAllocatorRange", alloc.MCSAllocatorRange, err.Error()))
		}
		if alloc.MCSLabelsPerProject <= 0 {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("securityAllocator.mcsLabelsPerProject", alloc.MCSLabelsPerProject, "must be a positive integer"))
		}

	} else {
		validationResults.AddWarnings(fielderrors.NewFieldInvalid("securityAllocator", "null", "allocation of UIDs and MCS labels to a project must be done manually"))

	}

	return validationResults
}

func ValidateRoutingConfig(config api.RoutingConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.Subdomain) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("subdomain"))
	} else if !kuval.IsDNS1123Subdomain(config.Subdomain) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("subdomain", config.Subdomain, "must be a valid subdomain"))
	}

	return allErrs
}

func ValidateAPIServerExtendedArguments(config api.ExtendedArguments) fielderrors.ValidationErrorList {
	return ValidateExtendedArguments(config, kapp.NewAPIServer().AddFlags)
}

func ValidateControllerExtendedArguments(config api.ExtendedArguments) fielderrors.ValidationErrorList {
	return ValidateExtendedArguments(config, cmapp.NewCMServer().AddFlags)
}
