package validation

import (
	"fmt"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"

	apiserveroptions "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	controlleroptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	kvalidation "k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/serviceaccount"
	knet "k8s.io/kubernetes/pkg/util/net"
	"k8s.io/kubernetes/pkg/util/sets"
	kuval "k8s.io/kubernetes/pkg/util/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/security/mcs"
	"github.com/openshift/origin/pkg/security/uid"
	"github.com/openshift/origin/pkg/util/labelselector"
)

// TODO: this should just be two return arrays, no need to be clever
type ValidationResults struct {
	Warnings field.ErrorList
	Errors   field.ErrorList
}

func (r *ValidationResults) Append(additionalResults ValidationResults) {
	r.AddErrors(additionalResults.Errors...)
	r.AddWarnings(additionalResults.Warnings...)
}

func (r *ValidationResults) AddErrors(errors ...*field.Error) {
	if len(errors) == 0 {
		return
	}
	r.Errors = append(r.Errors, errors...)
}

func (r *ValidationResults) AddWarnings(warnings ...*field.Error) {
	if len(warnings) == 0 {
		return
	}
	r.Warnings = append(r.Warnings, warnings...)
}

func ValidateMasterConfig(config *api.MasterConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if _, urlErrs := ValidateURL(config.MasterPublicURL, fldPath.Child("masterPublicURL")); len(urlErrs) > 0 {
		validationResults.AddErrors(urlErrs...)
	}

	switch {
	case config.ControllerLeaseTTL > 300,
		config.ControllerLeaseTTL < -1,
		config.ControllerLeaseTTL > 0 && config.ControllerLeaseTTL < 10:
		validationResults.AddErrors(field.Invalid(fldPath.Child("controllerLeaseTTL"), config.ControllerLeaseTTL, "TTL must be -1 (disabled), 0 (default), or between 10 and 300 seconds"))
	}

	validationResults.AddErrors(ValidateDisabledFeatures(config.DisabledFeatures, fldPath.Child("disabledFeatures"))...)

	if config.AssetConfig != nil {
		assetConfigPath := fldPath.Child("assetConfig")
		validationResults.Append(ValidateAssetConfig(config.AssetConfig, assetConfigPath))
		colocated := config.AssetConfig.ServingInfo.BindAddress == config.ServingInfo.BindAddress
		if colocated {
			publicURL, _ := url.Parse(config.AssetConfig.PublicURL)
			if publicURL.Path == "/" {
				validationResults.AddErrors(field.Invalid(assetConfigPath.Child("publicURL"), config.AssetConfig.PublicURL, "path can not be / when colocated with master API"))
			}

			// Warn if they have customized the asset certificates in ways that will be ignored
			if !reflect.DeepEqual(config.AssetConfig.ServingInfo.ServerCert, config.ServingInfo.ServerCert) ||
				!reflect.DeepEqual(config.AssetConfig.ServingInfo.NamedCertificates, config.ServingInfo.NamedCertificates) {
				validationResults.AddWarnings(field.Invalid(assetConfigPath.Child("servingInfo"), "<not displayed>", "changes to assetConfig certificate configuration are not used when colocated with master API"))
			}
		}

		if config.OAuthConfig != nil {
			if config.OAuthConfig.AssetPublicURL != config.AssetConfig.PublicURL {
				validationResults.AddErrors(
					field.Invalid(assetConfigPath.Child("publicURL"), config.AssetConfig.PublicURL, "must match oauthConfig.assetPublicURL"),
					field.Invalid(fldPath.Child("oauthConfig", "assetPublicURL"), config.OAuthConfig.AssetPublicURL, "must match assetConfig.publicURL"),
				)
			}
		}

		// TODO warn when the CORS list does not include the assetConfig.publicURL host:port
		// only warn cause they could handle CORS headers themselves in a proxy
	}

	if config.DNSConfig != nil {
		dnsConfigPath := fldPath.Child("dnsConfig")
		validationResults.AddErrors(ValidateHostPort(config.DNSConfig.BindAddress, dnsConfigPath.Child("bindAddress"))...)
		switch config.DNSConfig.BindNetwork {
		case "tcp", "tcp4", "tcp6":
		default:
			validationResults.AddErrors(field.Invalid(dnsConfigPath.Child("bindNetwork"), config.DNSConfig.BindNetwork, "must be 'tcp', 'tcp4', or 'tcp6'"))
		}
	}

	if config.EtcdConfig != nil {
		etcdConfigErrs := ValidateEtcdConfig(config.EtcdConfig, fldPath.Child("etcdConfig"))
		validationResults.Append(etcdConfigErrs)

		if len(etcdConfigErrs.Errors) == 0 {
			// Validate the etcdClientInfo with the internal etcdConfig
			validationResults.AddErrors(ValidateEtcdConnectionInfo(config.EtcdClientInfo, config.EtcdConfig, fldPath.Child("etcdClientInfo"))...)
		} else {
			// Validate the etcdClientInfo by itself
			validationResults.AddErrors(ValidateEtcdConnectionInfo(config.EtcdClientInfo, nil, fldPath.Child("etcdClientInfo"))...)
		}
	} else {
		// Validate the etcdClientInfo by itself
		validationResults.AddErrors(ValidateEtcdConnectionInfo(config.EtcdClientInfo, nil, fldPath.Child("etcdClientInfo"))...)
	}
	validationResults.AddErrors(ValidateEtcdStorageConfig(config.EtcdStorageConfig, fldPath.Child("etcdStorageConfig"))...)

	validationResults.AddErrors(ValidateImageConfig(config.ImageConfig, fldPath.Child("imageConfig"))...)

	validationResults.AddErrors(ValidateImagePolicyConfig(config.ImagePolicyConfig, fldPath.Child("imagePolicyConfig"))...)

	validationResults.AddErrors(ValidateKubeletConnectionInfo(config.KubeletClientInfo, fldPath.Child("kubeletClientInfo"))...)

	builtInKubernetes := config.KubernetesMasterConfig != nil
	if config.KubernetesMasterConfig != nil {
		validationResults.Append(ValidateKubernetesMasterConfig(config.KubernetesMasterConfig, fldPath.Child("kubernetesMasterConfig")))
	}
	if (config.KubernetesMasterConfig == nil) && (len(config.MasterClients.ExternalKubernetesKubeConfig) == 0) {
		validationResults.AddErrors(field.Invalid(fldPath.Child("kubernetesMasterConfig"), config.KubernetesMasterConfig, "either kubernetesMasterConfig or masterClients.externalKubernetesKubeConfig must have a value"))
	}
	if (config.KubernetesMasterConfig != nil) && (len(config.MasterClients.ExternalKubernetesKubeConfig) != 0) {
		validationResults.AddErrors(field.Invalid(fldPath.Child("kubernetesMasterConfig"), config.KubernetesMasterConfig, "kubernetesMasterConfig and masterClients.externalKubernetesKubeConfig are mutually exclusive"))
	}

	if len(config.NetworkConfig.ServiceNetworkCIDR) > 0 {
		if _, _, err := net.ParseCIDR(strings.TrimSpace(config.NetworkConfig.ServiceNetworkCIDR)); err != nil {
			validationResults.AddErrors(field.Invalid(fldPath.Child("networkConfig", "serviceNetworkCIDR"), config.NetworkConfig.ServiceNetworkCIDR, "must be a valid CIDR notation IP range (e.g. 172.30.0.0/16)"))
		} else if config.KubernetesMasterConfig != nil && len(config.KubernetesMasterConfig.ServicesSubnet) > 0 && config.KubernetesMasterConfig.ServicesSubnet != config.NetworkConfig.ServiceNetworkCIDR {
			validationResults.AddErrors(field.Invalid(fldPath.Child("networkConfig", "serviceNetworkCIDR"), config.NetworkConfig.ServiceNetworkCIDR, fmt.Sprintf("must match kubernetesMasterConfig.servicesSubnet value of %q", config.KubernetesMasterConfig.ServicesSubnet)))
		}
	}
	if len(config.NetworkConfig.ExternalIPNetworkCIDRs) > 0 {
		for i, s := range config.NetworkConfig.ExternalIPNetworkCIDRs {
			if strings.HasPrefix(s, "!") {
				s = s[1:]
			}
			if _, _, err := net.ParseCIDR(s); err != nil {
				validationResults.AddErrors(field.Invalid(fldPath.Child("networkConfig", "externalIPNetworkCIDRs").Index(i), config.NetworkConfig.ExternalIPNetworkCIDRs[i], "must be a valid CIDR notation IP range (e.g. 172.30.0.0/16) with an optional leading !"))
			}
		}
	}

	validationResults.AddErrors(ValidateIngressIPNetworkCIDR(config, fldPath.Child("networkConfig", "ingressIPNetworkCIDR").Index(0))...)

	validationResults.AddErrors(ValidateKubeConfig(config.MasterClients.OpenShiftLoopbackKubeConfig, fldPath.Child("masterClients", "openShiftLoopbackKubeConfig"))...)

	if len(config.MasterClients.ExternalKubernetesKubeConfig) > 0 {
		validationResults.AddErrors(ValidateKubeConfig(config.MasterClients.ExternalKubernetesKubeConfig, fldPath.Child("masterClients", "externalKubernetesKubeConfig"))...)
	}

	validationResults.AddErrors(ValidatePolicyConfig(config.PolicyConfig, fldPath.Child("policyConfig"))...)
	if config.OAuthConfig != nil {
		validationResults.Append(ValidateOAuthConfig(config.OAuthConfig, fldPath.Child("oauthConfig")))
	}

	validationResults.Append(ValidateServiceAccountConfig(config.ServiceAccountConfig, builtInKubernetes, fldPath.Child("serviceAccountConfig")))

	validationResults.Append(ValidateHTTPServingInfo(config.ServingInfo, fldPath.Child("servingInfo")))

	validationResults.Append(ValidateProjectConfig(config.ProjectConfig, fldPath.Child("projectConfig")))

	validationResults.AddErrors(ValidateRoutingConfig(config.RoutingConfig, fldPath.Child("routingConfig"))...)

	validationResults.Append(ValidateAPILevels(config.APILevels, api.KnownOpenShiftAPILevels, api.DeadOpenShiftAPILevels, fldPath.Child("apiLevels")))

	if config.AdmissionConfig.PluginConfig != nil {
		validationResults.AddErrors(ValidateAdmissionPluginConfig(config.AdmissionConfig.PluginConfig, fldPath.Child("admissionConfig", "pluginConfig"))...)
		validationResults.Append(ValidateAdmissionPluginConfigConflicts(config))
	}
	if len(config.AdmissionConfig.PluginOrderOverride) != 0 {
		validationResults.AddWarnings(field.Invalid(fldPath.Child("admissionConfig", "pluginOrderOverride"), config.AdmissionConfig.PluginOrderOverride, "specified admission ordering is being phased out.  Convert to DefaultAdmissionConfig in admissionConfig.pluginConfig."))
	}

	validationResults.Append(ValidateControllerConfig(config.ControllerConfig, fldPath.Child("controllerConfig")))

	validationResults.Append(ValidateAuditConfig(config.AuditConfig, fldPath.Child("auditConfig")))

	return validationResults
}

func ValidateAuditConfig(config api.AuditConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if len(config.AuditFilePath) == 0 {
		// for backwards compatibility reasons we can't error this out
		validationResults.AddWarnings(field.Required(fldPath.Child("auditFilePath"), "audit can now be logged to a separate file"))
	}
	if config.MaximumFileRetentionDays < 0 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("maximumFileRetentionDays"), config.MaximumFileRetentionDays, "must be greater than or equal to 0"))
	}
	if config.MaximumRetainedFiles < 0 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("maximumRetainedFiles"), config.MaximumRetainedFiles, "must be greater than or equal to 0"))
	}
	if config.MaximumFileSizeMegabytes < 0 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("maximumFileSizeMegabytes"), config.MaximumFileSizeMegabytes, "must be greater than or equal to 0"))
	}

	return validationResults
}

func ValidateControllerConfig(config api.ControllerConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if config.ServiceServingCert.Signer != nil {
		validationResults.AddErrors(ValidateCertInfo(*config.ServiceServingCert.Signer, true, fldPath.Child("serviceServingCert.signer"))...)
	}

	return validationResults
}

func ValidateAPILevels(apiLevels []string, knownAPILevels, deadAPILevels []string, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if len(apiLevels) == 0 {
		validationResults.AddErrors(field.Required(fldPath, ""))
	}

	deadLevels := sets.NewString(deadAPILevels...)
	knownLevels := sets.NewString(knownAPILevels...)
	for i, apiLevel := range apiLevels {
		idxPath := fldPath.Index(i)
		if deadLevels.Has(apiLevel) {
			validationResults.AddWarnings(field.Invalid(idxPath, apiLevel, "unsupported level"))
		}
		if !knownLevels.Has(apiLevel) {
			validationResults.AddWarnings(field.Invalid(idxPath, apiLevel, "unknown level"))
		}
	}

	return validationResults
}

func ValidateEtcdStorageConfig(config api.EtcdStorageConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateStorageVersionLevel(
		config.KubernetesStorageVersion,
		api.KnownKubernetesStorageVersionLevels,
		api.DeadKubernetesStorageVersionLevels,
		fldPath.Child("kubernetesStorageVersion"))...)
	allErrs = append(allErrs, ValidateStorageVersionLevel(
		config.OpenShiftStorageVersion,
		api.KnownOpenShiftStorageVersionLevels,
		api.DeadOpenShiftStorageVersionLevels,
		fldPath.Child("openShiftStorageVersion"))...)

	if strings.ContainsRune(config.KubernetesStoragePrefix, '%') {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("kubernetesStoragePrefix"), config.KubernetesStoragePrefix, "the '%' character may not be used in etcd path prefixes"))
	}
	if strings.ContainsRune(config.OpenShiftStoragePrefix, '%') {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("openShiftStoragePrefix"), config.OpenShiftStoragePrefix, "the '%' character may not be used in etcd path prefixes"))
	}

	return allErrs
}

func ValidateStorageVersionLevel(level string, knownAPILevels, deadAPILevels []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(level) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, ""))
		return allErrs
	}
	supportedLevels := sets.NewString(knownAPILevels...)
	supportedLevels.Delete(deadAPILevels...)
	if !supportedLevels.Has(level) {
		allErrs = append(allErrs, field.NotSupported(fldPath, level, supportedLevels.List()))
	}

	return allErrs
}

func ValidateServiceAccountConfig(config api.ServiceAccountConfig, builtInKubernetes bool, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	managedNames := sets.NewString(config.ManagedNames...)
	managedNamesPath := fldPath.Child("managedNames")
	if !managedNames.Has(bootstrappolicy.BuilderServiceAccountName) {
		validationResults.AddWarnings(field.Invalid(managedNamesPath, "", fmt.Sprintf("missing %q, which will require manual creation in each namespace before builds can run", bootstrappolicy.BuilderServiceAccountName)))
	}
	if !managedNames.Has(bootstrappolicy.DeployerServiceAccountName) {
		validationResults.AddWarnings(field.Invalid(managedNamesPath, "", fmt.Sprintf("missing %q, which will require manual creation in each namespace before deployments can run", bootstrappolicy.DeployerServiceAccountName)))
	}
	if builtInKubernetes && !managedNames.Has(bootstrappolicy.DefaultServiceAccountName) {
		validationResults.AddWarnings(field.Invalid(managedNamesPath, "", fmt.Sprintf("missing %q, which will prevent creation of pods that do not specify a valid service account", bootstrappolicy.DefaultServiceAccountName)))
	}

	for i, name := range config.ManagedNames {
		if reasons := kvalidation.ValidateServiceAccountName(name, false); len(reasons) != 0 {
			validationResults.AddErrors(field.Invalid(managedNamesPath.Index(i), name, strings.Join(reasons, ", ")))
		}
	}

	if len(config.PrivateKeyFile) > 0 {
		privateKeyFilePath := fldPath.Child("privateKeyFile")
		if fileErrs := ValidateFile(config.PrivateKeyFile, privateKeyFilePath); len(fileErrs) > 0 {
			validationResults.AddErrors(fileErrs...)
		} else if privateKey, err := serviceaccount.ReadPrivateKey(config.PrivateKeyFile); err != nil {
			validationResults.AddErrors(field.Invalid(privateKeyFilePath, config.PrivateKeyFile, err.Error()))
		} else if err := privateKey.Validate(); err != nil {
			validationResults.AddErrors(field.Invalid(privateKeyFilePath, config.PrivateKeyFile, err.Error()))
		}
	} else if builtInKubernetes {
		validationResults.AddWarnings(field.Invalid(fldPath.Child("privateKeyFile"), "", "no service account tokens will be generated, which could prevent builds and deployments from working"))
	}

	if len(config.PublicKeyFiles) == 0 {
		validationResults.AddWarnings(field.Invalid(fldPath.Child("publicKeyFiles"), "", "no service account tokens will be accepted by the API, which will prevent builds and deployments from working"))
	}
	for i, publicKeyFile := range config.PublicKeyFiles {
		idxPath := fldPath.Child("publicKeyFiles").Index(i)
		if fileErrs := ValidateFile(publicKeyFile, idxPath); len(fileErrs) > 0 {
			validationResults.AddErrors(fileErrs...)
		} else if _, err := serviceaccount.ReadPublicKey(publicKeyFile); err != nil {
			validationResults.AddErrors(field.Invalid(idxPath, publicKeyFile, err.Error()))
		}
	}

	if len(config.MasterCA) > 0 {
		validationResults.AddErrors(ValidateFile(config.MasterCA, fldPath.Child("masterCA"))...)
	} else if builtInKubernetes {
		validationResults.AddWarnings(field.Invalid(fldPath.Child("masterCA"), "", "master CA information will not be automatically injected into pods, which will prevent verification of the API server from inside a pod"))
	}

	return validationResults
}

func ValidateAssetConfig(config *api.AssetConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.Append(ValidateHTTPServingInfo(config.ServingInfo, fldPath.Child("servingInfo")))

	if len(config.LogoutURL) > 0 {
		_, urlErrs := ValidateURL(config.LogoutURL, fldPath.Child("logoutURL"))
		if len(urlErrs) > 0 {
			validationResults.AddErrors(urlErrs...)
		}
	}

	urlObj, urlErrs := ValidateURL(config.PublicURL, fldPath.Child("publicURL"))
	if len(urlErrs) > 0 {
		validationResults.AddErrors(urlErrs...)
	}
	if urlObj != nil {
		if !strings.HasSuffix(urlObj.Path, "/") {
			validationResults.AddErrors(field.Invalid(fldPath.Child("publicURL"), config.PublicURL, "must have a trailing slash in path"))
		}
	}

	if _, urlErrs := ValidateURL(config.MasterPublicURL, fldPath.Child("masterPublicURL")); len(urlErrs) > 0 {
		validationResults.AddErrors(urlErrs...)
	}

	if len(config.LoggingPublicURL) > 0 {
		if _, loggingURLErrs := ValidateSecureURL(config.LoggingPublicURL, fldPath.Child("loggingPublicURL")); len(loggingURLErrs) > 0 {
			validationResults.AddErrors(loggingURLErrs...)
		}
	} else {
		validationResults.AddWarnings(field.Invalid(fldPath.Child("loggingPublicURL"), "", "required to view aggregated container logs in the console"))
	}

	if len(config.MetricsPublicURL) > 0 {
		if _, metricsURLErrs := ValidateSecureURL(config.MetricsPublicURL, fldPath.Child("metricsPublicURL")); len(metricsURLErrs) > 0 {
			validationResults.AddErrors(metricsURLErrs...)
		}
	} else {
		validationResults.AddWarnings(field.Invalid(fldPath.Child("metricsPublicURL"), "", "required to view cluster metrics in the console"))
	}

	for i, scriptFile := range config.ExtensionScripts {
		validationResults.AddErrors(ValidateFile(scriptFile, fldPath.Child("extensionScripts").Index(i))...)
	}

	for i, stylesheetFile := range config.ExtensionStylesheets {
		validationResults.AddErrors(ValidateFile(stylesheetFile, fldPath.Child("extensionStylesheets").Index(i))...)
	}

	nameTaken := map[string]bool{}
	for i, extConfig := range config.Extensions {
		idxPath := fldPath.Child("extensions").Index(i)
		extConfigErrors := ValidateAssetExtensionsConfig(extConfig, idxPath)
		validationResults.AddErrors(extConfigErrors...)
		if nameTaken[extConfig.Name] {
			dupError := field.Invalid(idxPath.Child("name"), extConfig.Name, "duplicate extension name")
			validationResults.AddErrors(dupError)
		} else {
			nameTaken[extConfig.Name] = true
		}
	}

	return validationResults
}

var extNameExp = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func ValidateAssetExtensionsConfig(extConfig api.AssetExtensionsConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateDir(extConfig.SourceDirectory, fldPath.Child("sourceDirectory"))...)

	if len(extConfig.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
	} else if !extNameExp.MatchString(extConfig.Name) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), extConfig.Name, fmt.Sprintf("does not match %v", extNameExp)))
	}

	return allErrs
}

func ValidateImageConfig(config api.ImageConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(config.Format) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("format"), ""))
	}

	return allErrs
}

func ValidateImagePolicyConfig(config api.ImagePolicyConfig, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if config.MaxImagesBulkImportedPerRepository == 0 || config.MaxImagesBulkImportedPerRepository < -1 {
		errs = append(errs, field.Invalid(fldPath.Child("maxImagesBulkImportedPerRepository"), config.MaxImagesBulkImportedPerRepository, "must be a positive integer or -1"))
	}
	if config.ScheduledImageImportMinimumIntervalSeconds <= 0 {
		errs = append(errs, field.Invalid(fldPath.Child("scheduledImageImportMinimumIntervalSeconds"), config.ScheduledImageImportMinimumIntervalSeconds, "must be a positive integer"))
	}
	if config.MaxScheduledImageImportsPerMinute == 0 || config.MaxScheduledImageImportsPerMinute < -1 {
		errs = append(errs, field.Invalid(fldPath.Child("maxScheduledImageImportsPerMinute"), config.MaxScheduledImageImportsPerMinute, "must be a positive integer or -1"))
	}
	return errs
}

func ValidateKubeletConnectionInfo(config api.KubeletConnectionInfo, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config.Port == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("port"), ""))
	}

	if len(config.CA) > 0 {
		allErrs = append(allErrs, ValidateFile(config.CA, fldPath.Child("ca"))...)
	}
	allErrs = append(allErrs, ValidateCertInfo(config.ClientCert, false, fldPath)...)

	return allErrs
}

func ValidateKubernetesMasterConfig(config *api.KubernetesMasterConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if len(config.MasterIP) > 0 {
		validationResults.AddErrors(ValidateSpecifiedIP(config.MasterIP, fldPath.Child("masterIP"))...)
	}

	if config.MasterCount == 0 || config.MasterCount < -1 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("masterCount"), config.MasterCount, "must be a positive integer or -1"))
	}

	validationResults.AddErrors(ValidateCertInfo(config.ProxyClientInfo, false, fldPath.Child("proxyClientInfo"))...)
	if len(config.ProxyClientInfo.CertFile) == 0 && len(config.ProxyClientInfo.KeyFile) == 0 {
		validationResults.AddWarnings(field.Invalid(fldPath.Child("proxyClientInfo"), "", "if no client certificate is specified, TLS pods and services cannot validate requests came from the proxy"))
	}

	if len(config.ServicesSubnet) > 0 {
		if _, _, err := net.ParseCIDR(strings.TrimSpace(config.ServicesSubnet)); err != nil {
			validationResults.AddErrors(field.Invalid(fldPath.Child("servicesSubnet"), config.ServicesSubnet, "must be a valid CIDR notation IP range (e.g. 172.30.0.0/16)"))
		}
	}

	if len(config.ServicesNodePortRange) > 0 {
		if _, err := knet.ParsePortRange(strings.TrimSpace(config.ServicesNodePortRange)); err != nil {
			validationResults.AddErrors(field.Invalid(fldPath.Child("servicesNodePortRange"), config.ServicesNodePortRange, "must be a valid port range (e.g. 30000-32000)"))
		}
	}

	if len(config.SchedulerConfigFile) > 0 {
		validationResults.AddErrors(ValidateFile(config.SchedulerConfigFile, fldPath.Child("schedulerConfigFile"))...)
	}

	for i, nodeName := range config.StaticNodeNames {
		if len(nodeName) == 0 {
			validationResults.AddErrors(field.Invalid(fldPath.Child("staticNodeName").Index(i), nodeName, "may not be empty"))
		} else {
			validationResults.AddWarnings(field.Invalid(fldPath.Child("staticNodeName").Index(i), nodeName, "static nodes are not supported"))
		}
	}

	if len(config.PodEvictionTimeout) > 0 {
		if _, err := time.ParseDuration(config.PodEvictionTimeout); err != nil {
			validationResults.AddErrors(field.Invalid(fldPath.Child("podEvictionTimeout"), config.PodEvictionTimeout, "must be a valid time duration string (e.g. '300ms' or '2m30s'). Valid time units are 'ns', 'us', 'ms', 's', 'm', 'h'"))
		}
	}

	for group, versions := range config.DisabledAPIGroupVersions {
		keyPath := fldPath.Child("disabledAPIGroupVersions").Key(group)
		if !api.KnownKubeAPIGroups.Has(group) {
			validationResults.AddWarnings(field.NotSupported(keyPath, group, api.KnownKubeAPIGroups.List()))
			continue
		}

		allowedVersions := sets.NewString(api.KubeAPIGroupsToAllowedVersions[group]...)
		for i, version := range versions {
			if version == "*" {
				continue
			}

			if !allowedVersions.Has(version) {
				validationResults.AddWarnings(field.NotSupported(keyPath.Index(i), version, allowedVersions.List()))
			}
		}
	}

	if config.AdmissionConfig.PluginConfig != nil {
		validationResults.AddErrors(ValidateAdmissionPluginConfig(config.AdmissionConfig.PluginConfig, fldPath.Child("admissionConfig", "pluginConfig"))...)
	}
	if len(config.AdmissionConfig.PluginOrderOverride) != 0 {
		validationResults.AddWarnings(field.Invalid(fldPath.Child("admissionConfig", "pluginOrderOverride"), config.AdmissionConfig.PluginOrderOverride, "specified admission ordering is being phased out.  Convert to DefaultAdmissionConfig in admissionConfig.pluginConfig."))
	}

	validationResults.Append(ValidateAPIServerExtendedArguments(config.APIServerArguments, fldPath.Child("apiServerArguments")))
	validationResults.AddErrors(ValidateControllerExtendedArguments(config.ControllerArguments, fldPath.Child("controllerArguments"))...)

	return validationResults
}

func ValidatePolicyConfig(config api.PolicyConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateFile(config.BootstrapPolicyFile, fldPath.Child("bootstrapPolicyFile"))...)
	allErrs = append(allErrs, ValidateNamespace(config.OpenShiftSharedResourcesNamespace, fldPath.Child("openShiftSharedResourcesNamespace"))...)
	allErrs = append(allErrs, ValidateNamespace(config.OpenShiftInfrastructureNamespace, fldPath.Child("openShiftInfrastructureNamespace"))...)

	for i, matchingRule := range config.UserAgentMatchingConfig.DeniedClients {
		_, err := regexp.Compile(matchingRule.Regex)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("userAgentMatchingConfig", "deniedClients").Index(i), matchingRule.Regex, err.Error()))
		}
	}
	for i, matchingRule := range config.UserAgentMatchingConfig.RequiredClients {
		_, err := regexp.Compile(matchingRule.Regex)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("userAgentMatchingConfig", "requiredClients").Index(i), matchingRule.Regex, err.Error()))
		}
	}

	return allErrs
}

func ValidateProjectConfig(config api.ProjectConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if _, _, err := api.ParseNamespaceAndName(config.ProjectRequestTemplate); err != nil {
		validationResults.AddErrors(field.Invalid(fldPath.Child("projectRequestTemplate"), config.ProjectRequestTemplate, "must be in the form: namespace/templateName"))
	}

	if len(config.DefaultNodeSelector) > 0 {
		_, err := labelselector.Parse(config.DefaultNodeSelector)
		if err != nil {
			validationResults.AddErrors(field.Invalid(fldPath.Child("defaultNodeSelector"), config.DefaultNodeSelector, "must be a valid label selector"))
		}
	}

	if alloc := config.SecurityAllocator; alloc != nil {
		securityAllocatorPath := fldPath.Child("securityAllocator")
		if _, err := uid.ParseRange(alloc.UIDAllocatorRange); err != nil {
			validationResults.AddErrors(field.Invalid(securityAllocatorPath.Child("uidAllocatorRange"), alloc.UIDAllocatorRange, err.Error()))
		}
		if _, err := mcs.ParseRange(alloc.MCSAllocatorRange); err != nil {
			validationResults.AddErrors(field.Invalid(securityAllocatorPath.Child("mcsAllocatorRange"), alloc.MCSAllocatorRange, err.Error()))
		}
		if alloc.MCSLabelsPerProject <= 0 {
			validationResults.AddErrors(field.Invalid(securityAllocatorPath.Child("mcsLabelsPerProject"), alloc.MCSLabelsPerProject, "must be a positive integer"))
		}

	} else {
		validationResults.AddWarnings(field.Invalid(fldPath.Child("securityAllocator"), "null", "allocation of UIDs and MCS labels to a project must be done manually"))

	}

	return validationResults
}

func ValidateRoutingConfig(config api.RoutingConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(config.Subdomain) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("subdomain"), ""))
	} else if len(kuval.IsDNS1123Subdomain(config.Subdomain)) != 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("subdomain"), config.Subdomain, "must be a valid subdomain"))
	}

	return allErrs
}

func ValidateAPIServerExtendedArguments(config api.ExtendedArguments, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.AddErrors(ValidateExtendedArguments(config, apiserveroptions.NewAPIServer().AddFlags, fldPath)...)

	if len(config["admission-control"]) > 0 {
		validationResults.AddWarnings(field.Invalid(fldPath.Key("admission-control"), config["admission-control"], "specified admission ordering is being phased out.  Convert to DefaultAdmissionConfig in admissionConfig.pluginConfig."))
	}
	if len(config["admission-control-config-file"]) > 0 {
		validationResults.AddWarnings(field.Invalid(fldPath.Key("admission-control-config-file"), config["admission-control-config-file"], "specify a single admission control config file is being phased out.  Convert to admissionConfig.pluginConfig, one file per plugin."))
	}

	return validationResults
}

func ValidateControllerExtendedArguments(config api.ExtendedArguments, fldPath *field.Path) field.ErrorList {
	return ValidateExtendedArguments(config, controlleroptions.NewCMServer().AddFlags, fldPath)
}

func ValidateAdmissionPluginConfig(pluginConfig map[string]api.AdmissionPluginConfig, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for name, config := range pluginConfig {
		if len(config.Location) > 0 && config.Configuration != nil {
			allErrs = append(allErrs, field.Invalid(fieldPath.Key(name), "", "cannot specify both location and embedded config"))
		}
		if len(config.Location) == 0 && config.Configuration == nil {
			allErrs = append(allErrs, field.Invalid(fieldPath.Key(name), "", "must specify either a location or an embedded config"))
		}
	}
	return allErrs

}

func ValidateAdmissionPluginConfigConflicts(masterConfig *api.MasterConfig) ValidationResults {
	validationResults := ValidationResults{}

	if masterConfig.KubernetesMasterConfig != nil {
		// check for collisions between openshift and kube plugin config
		for pluginName, kubeConfig := range masterConfig.KubernetesMasterConfig.AdmissionConfig.PluginConfig {
			if openshiftConfig, exists := masterConfig.AdmissionConfig.PluginConfig[pluginName]; exists && !reflect.DeepEqual(kubeConfig, openshiftConfig) {
				validationResults.AddWarnings(field.Invalid(field.NewPath("kubernetesMasterConfig", "admissionConfig", "pluginConfig").Key(pluginName), masterConfig.AdmissionConfig.PluginConfig[pluginName], "conflicts with kubernetesMasterConfig.admissionConfig.pluginConfig.  Separate admission chains are being phased out.  Convert to admissionConfig.pluginConfig."))
			}
		}
	}

	return validationResults
}

func ValidateIngressIPNetworkCIDR(config *api.MasterConfig, fldPath *field.Path) (errors field.ErrorList) {
	cidr := config.NetworkConfig.IngressIPNetworkCIDR
	if len(cidr) == 0 {
		return
	}

	addError := func(errMessage string) {
		errors = append(errors, field.Invalid(fldPath, cidr, errMessage))
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		addError("must be a valid CIDR notation IP range (e.g. 172.46.0.0/16)")
		return
	}

	// TODO Detect cloud provider when not using built-in kubernetes
	kubeConfig := config.KubernetesMasterConfig
	noCloudProvider := kubeConfig != nil && (len(kubeConfig.ControllerArguments["cloud-provider"]) == 0 || kubeConfig.ControllerArguments["cloud-provider"][0] == "")

	if noCloudProvider {
		if api.CIDRsOverlap(cidr, config.NetworkConfig.ClusterNetworkCIDR) {
			addError("conflicts with cluster network CIDR")
		}
		if api.CIDRsOverlap(cidr, config.NetworkConfig.ServiceNetworkCIDR) {
			addError("conflicts with service network CIDR")
		}
	} else if !ipNet.IP.IsUnspecified() {
		addError("should not be provided when a cloud-provider is enabled")
	}

	return
}
