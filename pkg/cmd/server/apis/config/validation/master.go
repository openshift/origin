package validation

import (
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	knet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	kuval "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	auditvalidation "k8s.io/apiserver/pkg/apis/audit/validation"
	auditpolicy "k8s.io/apiserver/pkg/audit/policy"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
	apiserveroptions "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	kcmoptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	kvalidation "k8s.io/kubernetes/pkg/apis/core/validation"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/cm"
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

func ValidateMasterConfig(config *configapi.MasterConfig, fldPath *field.Path) ValidationResults {
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
			s = strings.TrimPrefix(s, "!")
			if _, _, err := net.ParseCIDR(s); err != nil {
				validationResults.AddErrors(field.Invalid(fldPath.Child("networkConfig", "externalIPNetworkCIDRs").Index(i), config.NetworkConfig.ExternalIPNetworkCIDRs[i], "must be a valid CIDR notation IP range (e.g. 172.30.0.0/16) with an optional leading !"))
			}
		}
	}

	validationResults.AddErrors(ValidateIngressIPNetworkCIDR(config, fldPath.Child("networkConfig", "ingressIPNetworkCIDR").Index(0))...)
	validationResults.Append(ValidateDeprecatedClusterNetworkConfig(config, fldPath.Child("networkConfig")))

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

	validationResults.Append(ValidateAPILevels(config.APILevels, configapi.KnownOpenShiftAPILevels, configapi.DeadOpenShiftAPILevels, fldPath.Child("apiLevels")))

	if config.AdmissionConfig.PluginConfig != nil {
		validationResults.Append(ValidateAdmissionPluginConfig(config.AdmissionConfig.PluginConfig, fldPath.Child("admissionConfig", "pluginConfig")))
	}
	if len(config.AdmissionConfig.PluginOrderOverride) != 0 {
		validationResults.AddWarnings(field.Invalid(fldPath.Child("admissionConfig", "pluginOrderOverride"), config.AdmissionConfig.PluginOrderOverride, "specified admission ordering is being phased out.  Convert to DefaultAdmissionConfig in admissionConfig.pluginConfig."))
	}

	validationResults.Append(ValidateControllerConfig(config.ControllerConfig, fldPath.Child("controllerConfig")))
	validationResults.Append(ValidateAuditConfig(config.AuditConfig, fldPath.Child("auditConfig")))
	validationResults.Append(ValidateMasterAuthConfig(config.AuthConfig, fldPath.Child("authConfig")))
	validationResults.Append(ValidateAggregatorConfig(config.AggregatorConfig, fldPath.Child("aggregatorConfig")))

	return validationResults
}

func ValidateMasterAuthConfig(config configapi.MasterAuthConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	for _, wta := range config.WebhookTokenAuthenticators {
		configFile := fldPath.Child("webhookTokenAuthenticators", "ConfigFile")
		if len(wta.ConfigFile) == 0 {
			validationResults.AddErrors(field.Required(configFile, ""))
		} else {
			validationResults.AddErrors(ValidateFile(wta.ConfigFile, configFile)...)
		}

		cacheTTL := fldPath.Child("webhookTokenAuthenticators", "cacheTTL")
		if len(wta.CacheTTL) == 0 {
			validationResults.AddErrors(field.Required(cacheTTL, ""))
		} else if ttl, err := time.ParseDuration(wta.CacheTTL); err != nil {
			validationResults.AddErrors(field.Invalid(cacheTTL, wta.CacheTTL, fmt.Sprintf("%v", err)))
		} else if ttl < 0 {
			validationResults.AddErrors(field.Invalid(cacheTTL, wta.CacheTTL, "cannot be less than zero"))
		}
	}

	if config.RequestHeader == nil {
		return validationResults
	}

	if len(config.RequestHeader.ClientCA) == 0 {
		validationResults.AddErrors(field.Required(fldPath.Child("requestHeader.clientCA"), "must be specified for a secure connection"))
	}
	if len(config.RequestHeader.ClientCommonNames) == 0 {
		validationResults.AddErrors(field.Required(fldPath.Child("requestHeader.clientCommonNames"), "must be specified for a secure connection"))
	}
	if len(config.RequestHeader.UsernameHeaders) == 0 {
		validationResults.AddErrors(field.Required(fldPath.Child("requestHeader.usernameHeaders"), "must be specified for a secure connection"))
	}
	if len(config.RequestHeader.GroupHeaders) == 0 {
		validationResults.AddErrors(field.Required(fldPath.Child("requestHeader.groupHeaders"), "must be specified for a secure connection"))
	}
	if len(config.RequestHeader.ExtraHeaderPrefixes) == 0 {
		validationResults.AddErrors(field.Required(fldPath.Child("requestHeader.extraHeaderPrefixes"), "must be specified for a secure connection"))
	}

	return validationResults
}

func ValidateAggregatorConfig(config configapi.AggregatorConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.AddErrors(ValidateCertInfo(config.ProxyClientInfo, false, fldPath.Child("proxyClientInfo"))...)
	if len(config.ProxyClientInfo.CertFile) == 0 && len(config.ProxyClientInfo.KeyFile) == 0 {
		validationResults.AddWarnings(field.Invalid(fldPath.Child("proxyClientInfo"), "", "if no client certificate is specified, the aggregator will be unable to proxy to remote servers"))
	}

	return validationResults
}

func ValidateAuditConfig(config configapi.AuditConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}
	if !config.Enabled {
		return validationResults
	}

	// TODO (soltysh): both of these should be turned into errors in 3.11
	if len(config.AuditFilePath) == 0 {
		// for backwards compatibility reasons we can't error this out
		validationResults.AddWarnings(field.Required(fldPath.Child("auditFilePath"), "audit needs to be logged to a separate file"))
	} else if !filepath.IsAbs(config.InternalAuditFilePath) {
		// for backwards compatibility reasons we can't error this out
		validationResults.AddWarnings(field.Invalid(fldPath.Child("auditFilePath"), config.InternalAuditFilePath, "must be absolute path"))
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

	// setting policy file will turn the advanced auditing on
	if config.PolicyConfiguration != nil && len(config.PolicyFile) > 0 {
		validationResults.AddErrors(field.Forbidden(fldPath.Child("policyFile"), "both policyFile and policyConfiguration cannot be specified"))
	}
	if config.PolicyConfiguration != nil || len(config.PolicyFile) > 0 {
		if config.PolicyConfiguration == nil {
			policy, err := auditpolicy.LoadPolicyFromFile(config.PolicyFile)
			if err != nil {
				validationResults.AddErrors(field.Invalid(fldPath.Child("policyFile"), config.PolicyFile, err.Error()))
			}
			if policy == nil || len(policy.Rules) == 0 {
				validationResults.AddErrors(field.Invalid(fldPath.Child("policyFile"), config.PolicyFile, "a policy file with 0 policies is not valid"))
			}
		} else {
			policyConfiguration, ok := config.PolicyConfiguration.(*auditinternal.Policy)
			if !ok {
				validationResults.AddErrors(field.Invalid(fldPath.Child("policyConfiguration"), config.PolicyConfiguration, "must be of type audit/v1beta1.Policy"))
			} else {
				if err := auditvalidation.ValidatePolicy(policyConfiguration); err != nil {
					validationResults.AddErrors(field.Invalid(fldPath.Child("policyConfiguration"), config.PolicyConfiguration, err.ToAggregate().Error()))
				}
				if len(policyConfiguration.Rules) == 0 {
					validationResults.AddErrors(field.Invalid(fldPath.Child("policyConfiguration"), config.PolicyFile, "a policy configuration with 0 policies is not valid"))
				}
			}
		}

		if len(config.AuditFilePath) == 0 {
			validationResults.AddErrors(field.Required(fldPath.Child("auditFilePath"), "advanced audit requires a separate log file"))
		}
		switch config.LogFormat {
		case configapi.LogFormatLegacy, configapi.LogFormatJson:
			// ok
		default:
			validationResults.AddErrors(field.NotSupported(fldPath.Child("logFormat"), config.LogFormat, []string{string(configapi.LogFormatLegacy), string(configapi.LogFormatJson)}))
		}

		if len(config.WebHookKubeConfig) > 0 {
			switch config.WebHookMode {
			case configapi.WebHookModeBatch, configapi.WebHookModeBlocking:
				// ok
			default:
				validationResults.AddErrors(field.NotSupported(fldPath.Child("webHookMode"), config.WebHookMode, []string{string(configapi.WebHookModeBatch), string(configapi.WebHookModeBlocking)}))
			}
			loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
			loadingRules.ExplicitPath = config.WebHookKubeConfig
			loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
			if _, err := loader.ClientConfig(); err != nil {
				validationResults.AddErrors(field.Invalid(fldPath.Child("webHookKubeConfig"), config.WebHookKubeConfig, err.Error()))
			}
		} else if len(config.WebHookMode) > 0 {
			validationResults.AddErrors(field.Required(fldPath.Child("webHookKubeConfig"), "must be specified when webHookMode is set"))
		}
	}

	return validationResults
}

func ValidateControllerConfig(config configapi.ControllerConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if election := config.Election; election != nil {
		if len(election.LockName) == 0 {
			validationResults.AddErrors(field.Invalid(fldPath.Child("election", "lockName"), election.LockName, "may not be empty"))
		}
		for _, msg := range kvalidation.ValidateServiceName(election.LockName, false) {
			validationResults.AddErrors(field.Invalid(fldPath.Child("election", "lockName"), election.LockName, msg))
		}
		if len(election.LockNamespace) == 0 {
			validationResults.AddErrors(field.Invalid(fldPath.Child("election", "lockNamespace"), election.LockNamespace, "may not be empty"))
		}
		for _, msg := range kvalidation.ValidateNamespaceName(election.LockNamespace, false) {
			validationResults.AddErrors(field.Invalid(fldPath.Child("election", "lockNamespace"), election.LockNamespace, msg))
		}
		if len(election.LockResource.Resource) == 0 {
			validationResults.AddErrors(field.Invalid(fldPath.Child("election", "lockResource", "resource"), election.LockResource.Resource, "may not be empty"))
		}
	}
	if config.ServiceServingCert.Signer == nil {
		validationResults.AddWarnings(field.Required(fldPath.Child("serviceServingCert", "signer"), "required for the service serving cert signer; automatic serving certificate signing will fail"))
	} else {
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

func ValidateEtcdStorageConfig(config configapi.EtcdStorageConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateStorageVersionLevel(
		config.KubernetesStorageVersion,
		configapi.KnownKubernetesStorageVersionLevels,
		configapi.DeadKubernetesStorageVersionLevels,
		fldPath.Child("kubernetesStorageVersion"))...)
	allErrs = append(allErrs, ValidateStorageVersionLevel(
		config.OpenShiftStorageVersion,
		configapi.KnownOpenShiftStorageVersionLevels,
		configapi.DeadOpenShiftStorageVersionLevels,
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

func ValidateServiceAccountConfig(config configapi.ServiceAccountConfig, builtInKubernetes bool, fldPath *field.Path) ValidationResults {
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
		} else if _, err := cert.PrivateKeyFromFile(config.PrivateKeyFile); err != nil {
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
		} else if _, err := cert.PublicKeysFromFile(publicKeyFile); err != nil {
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

func ValidateImageConfig(config configapi.ImageConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(config.Format) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("format"), ""))
	}

	return allErrs
}

func ValidateImagePolicyConfig(config configapi.ImagePolicyConfig, fldPath *field.Path) field.ErrorList {
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
	if config.AllowedRegistriesForImport != nil {
		for i, registry := range *config.AllowedRegistriesForImport {
			if len(registry.DomainName) == 0 {
				errs = append(errs, field.Invalid(fldPath.Index(i).Child("allowedRegistriesForImport", "domainName"), registry.DomainName, "cannot be an empty string"))
			}
			parts := strings.Split(registry.DomainName, ":")
			// Check for ':8080'
			if len(parts) == 0 || len(parts[0]) == 0 {
				errs = append(errs, field.Invalid(fldPath.Index(i).Child("allowedRegistriesForImport", "domainName"), registry.DomainName, "invalid domain specified, must be registry.url.local[:port]"))
			}
			// Check for 'foo:bar:1234'
			if len(parts) > 2 {
				errs = append(errs, field.Invalid(fldPath.Index(i).Child("allowedRegistriesForImport", "domainName"), registry.DomainName, "invalid format, must be registry.url.local[:port]"))
			}
			// Check for 'foo:bar'
			if len(parts) == 2 {
				if _, err := strconv.Atoi(parts[1]); err != nil {
					errs = append(errs, field.Invalid(fldPath.Index(i).Child("allowedRegistriesForImport", "domainName"), registry.DomainName, "invalid port format, must be a number"))
				}
			}
		}
	}
	return errs
}

func ValidateKubeletConnectionInfo(config configapi.KubeletConnectionInfo, fldPath *field.Path) field.ErrorList {
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

func ValidateKubernetesMasterConfig(config *configapi.KubernetesMasterConfig, fldPath *field.Path) ValidationResults {
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
		if !configapi.KnownKubeAPIGroups.Has(group) {
			validationResults.AddWarnings(field.NotSupported(keyPath, group, configapi.KnownKubeAPIGroups.List()))
			continue
		}

		allowedVersions := sets.NewString(configapi.KubeAPIGroupsToAllowedVersions[group]...)
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
		validationResults.AddErrors(field.Invalid(fldPath.Child("admissionConfig", "pluginConfig"), config.AdmissionConfig.PluginConfig, "separate admission chains are no longer allowed.  Convert to admissionConfig.pluginConfig."))
	}
	if len(config.AdmissionConfig.PluginOrderOverride) != 0 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("admissionConfig", "pluginOrderOverride"), config.AdmissionConfig.PluginOrderOverride, "separate admission chains are no longer allowed.  Convert to DefaultAdmissionConfig in admissionConfig.pluginConfig."))
	}

	validationResults.Append(ValidateAPIServerExtendedArguments(config.APIServerArguments, fldPath.Child("apiServerArguments")))
	validationResults.AddErrors(ValidateControllerExtendedArguments(config.ControllerArguments, fldPath.Child("controllerArguments"))...)

	return validationResults
}

func ValidatePolicyConfig(config configapi.PolicyConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

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

func ValidateProjectConfig(config configapi.ProjectConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if _, _, err := configapi.ParseNamespaceAndName(config.ProjectRequestTemplate); err != nil {
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

func ValidateRoutingConfig(config configapi.RoutingConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(config.Subdomain) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("subdomain"), ""))
	} else if len(kuval.IsDNS1123Subdomain(config.Subdomain)) != 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("subdomain"), config.Subdomain, "must be a valid subdomain"))
	}

	return allErrs
}

func ValidateAPIServerExtendedArguments(config configapi.ExtendedArguments, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.AddErrors(ValidateExtendedArguments(config, apiserveroptions.NewServerRunOptions().AddFlags, fldPath)...)

	if len(config["admission-control"]) > 0 {
		validationResults.AddWarnings(field.Invalid(fldPath.Key("admission-control"), config["admission-control"], "specified admission ordering is being phased out.  Convert to DefaultAdmissionConfig in admissionConfig.pluginConfig."))
	}
	if len(config["admission-control-config-file"]) > 0 {
		validationResults.AddWarnings(field.Invalid(fldPath.Key("admission-control-config-file"), config["admission-control-config-file"], "specify a single admission control config file is being phased out.  Convert to admissionConfig.pluginConfig, one file per plugin."))
	}

	return validationResults
}

func ValidateControllerExtendedArguments(config configapi.ExtendedArguments, fldPath *field.Path) field.ErrorList {
	return ValidateExtendedArguments(config, cm.OriginControllerManagerAddFlags(kcmoptions.NewCMServer()), fldPath)
}

// deprecatedAdmissionPluginNames returns the set of admission plugin names that are deprecated from use.
func deprecatedAdmissionPluginNames() sets.String {
	return sets.NewString("openshift.io/OriginResourceQuota")
}

func ValidateAdmissionPluginConfig(pluginConfig map[string]*configapi.AdmissionPluginConfig, fieldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	deprecatedPlugins := deprecatedAdmissionPluginNames()

	for name, config := range pluginConfig {
		if deprecatedPlugins.Has(name) {
			validationResults.AddWarnings(field.Invalid(fieldPath.Key(name), "", "specified admission plugin is deprecated"))
		}
		if len(config.Location) > 0 && config.Configuration != nil {
			validationResults.AddErrors(field.Invalid(fieldPath.Key(name), "", "cannot specify both location and embedded config"))
		}
		if len(config.Location) == 0 && config.Configuration == nil {
			validationResults.AddErrors(field.Invalid(fieldPath.Key(name), "", "must specify either a location or an embedded config"))
		}
	}
	return validationResults

}

func ValidateIngressIPNetworkCIDR(config *configapi.MasterConfig, fldPath *field.Path) (errors field.ErrorList) {
	cidr := config.NetworkConfig.IngressIPNetworkCIDR
	if len(cidr) == 0 {
		return
	}

	addError := func(errMessage string) {
		errors = append(errors, field.Invalid(fldPath, cidr, errMessage))
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		addError(fmt.Sprintf("must be a valid CIDR notation IP range (e.g. %s)", configapi.DefaultIngressIPNetworkCIDR))
		return
	}

	// TODO Detect cloud provider when not using built-in kubernetes
	kubeConfig := config.KubernetesMasterConfig
	noCloudProvider := kubeConfig != nil && (len(kubeConfig.ControllerArguments["cloud-provider"]) == 0 || kubeConfig.ControllerArguments["cloud-provider"][0] == "")

	if noCloudProvider {
		for _, entry := range config.NetworkConfig.ClusterNetworks {
			if configapi.CIDRsOverlap(cidr, entry.CIDR) {
				addError(fmt.Sprintf("conflicts with cluster network CIDR: %s", entry.CIDR))
			}
		}
		if configapi.CIDRsOverlap(cidr, config.NetworkConfig.ServiceNetworkCIDR) {
			addError("conflicts with service network CIDR")
		}
	} else if !ipNet.IP.IsUnspecified() {
		addError("should not be provided when a cloud-provider is enabled")
	}

	return
}

func ValidateDeprecatedClusterNetworkConfig(config *configapi.MasterConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if len(config.NetworkConfig.ClusterNetworks) > 1 {
		if config.NetworkConfig.DeprecatedHostSubnetLength != 0 {
			validationResults.AddErrors(field.Invalid(fldPath.Child("hostSubnetLength"), config.NetworkConfig.DeprecatedHostSubnetLength, "cannot set hostSubnetLength and clusterNetworks, please use clusterNetworks"))
		}
		if len(config.NetworkConfig.DeprecatedClusterNetworkCIDR) != 0 {
			validationResults.AddErrors(field.Invalid(fldPath.Child("clusterNetworkCIDR"), config.NetworkConfig.DeprecatedClusterNetworkCIDR, "cannot set clusterNetworkCIDR and clusterNetworks, please use clusterNetworks"))
		}

	} else if len(config.NetworkConfig.ClusterNetworks) == 1 {
		if config.NetworkConfig.DeprecatedHostSubnetLength != config.NetworkConfig.ClusterNetworks[0].HostSubnetLength && config.NetworkConfig.DeprecatedHostSubnetLength != 0 {
			validationResults.AddErrors(field.Invalid(fldPath.Child("hostSubnetLength"), config.NetworkConfig.DeprecatedHostSubnetLength, "cannot set hostSubnetLength and clusterNetworks, please use clusterNetworks"))
		}
		if config.NetworkConfig.DeprecatedClusterNetworkCIDR != config.NetworkConfig.ClusterNetworks[0].CIDR && len(config.NetworkConfig.DeprecatedClusterNetworkCIDR) != 0 {
			validationResults.AddErrors(field.Invalid(fldPath.Child("clusterNetworkCIDR"), config.NetworkConfig.DeprecatedClusterNetworkCIDR, "cannot set clusterNetworkCIDR and clusterNetworks, please use clusterNetworks"))
		}
	}

	return validationResults
}
