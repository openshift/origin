package validation

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	kvalidation "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/cmd/server/api"
)

func ValidateBindAddress(bindAddress string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(bindAddress) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("bindAddress"))
	} else if _, _, err := net.SplitHostPort(bindAddress); err != nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("bindAddress", bindAddress, "must be a host:port"))
	}

	return allErrs
}

func ValidateServingInfo(info api.ServingInfo) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateBindAddress(info.BindAddress)...)
	allErrs = append(allErrs, ValidateCertInfo(info.ServerCert)...)

	if len(info.ClientCA) > 0 {
		if (len(info.ServerCert.CertFile) == 0) || (len(info.ServerCert.KeyFile) == 0) {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("clientCA", info.ClientCA, "cannot specify a clientCA without a certFile"))
		}

		allErrs = append(allErrs, ValidateFile(info.ClientCA, "clientCA")...)
	}

	return allErrs
}

func ValidateKubeConfig(path string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateFile(path, field)...)
	// TODO: load and parse

	return allErrs
}

func ValidateKubernetesMasterConfig(config *api.KubernetesMasterConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.MasterIP) > 0 {
		allErrs = append(allErrs, ValidateSpecifiedIP(config.MasterIP, "masterIP")...)
	}

	if len(config.ServicesSubnet) > 0 {
		if _, _, err := net.ParseCIDR(strings.TrimSpace(config.ServicesSubnet)); err != nil {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("servicesSubnet", config.ServicesSubnet, "must be a valid CIDR notation IP range (e.g. 172.30.17.0/24)"))
		}
	}

	if len(config.SchedulerConfigFile) > 0 {
		allErrs = append(allErrs, ValidateFile(config.SchedulerConfigFile, "schedulerConfigFile")...)
	}

	return allErrs
}

func ValidateOAuthConfig(config *api.OAuthConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.MasterURL) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("masterURL"))
	}

	if len(config.MasterPublicURL) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("masterPublicURL"))
	}

	if len(config.AssetPublicURL) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("assetPublicURL"))
	}

	if config.SessionConfig != nil {
		allErrs = append(allErrs, ValidateSessionConfig(config.SessionConfig).Prefix("sessionConfig")...)
	}

	allErrs = append(allErrs, ValidateGrantConfig(config.GrantConfig).Prefix("grantConfig")...)

	redirectingIdentityProviders := []int{}
	for i, identityProvider := range config.IdentityProviders {
		if identityProvider.Usage.UseAsLogin {
			redirectingIdentityProviders = append(redirectingIdentityProviders, i)

			if api.IsPasswordAuthenticator(identityProvider) {
				if config.SessionConfig == nil {
					allErrs = append(allErrs, fielderrors.NewFieldInvalid("sessionConfig", config, "sessionConfig is required if a password identity provider is used for browser based login"))
				}
			}
		}

		allErrs = append(allErrs, ValidateIdentityProvider(identityProvider).Prefix(fmt.Sprintf("identityProvider[%d]", i))...)
	}

	if len(redirectingIdentityProviders) > 1 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("identityProviders", config.IdentityProviders, fmt.Sprintf("only one identity provider can support login for a browser, found: %v", redirectingIdentityProviders)))
	}

	return allErrs
}

func ValidateIdentityProvider(identityProvider api.IdentityProvider) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(identityProvider.Usage.ProviderName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("usage.providerName"))
	}

	if !api.IsIdentityProviderType(identityProvider.Provider) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("provider", identityProvider.Provider, fmt.Sprintf("%v is invalid in this context", identityProvider.Provider)))
	} else {
		switch provider := identityProvider.Provider.Object.(type) {
		case (*api.RequestHeaderIdentityProvider):
			if len(provider.ClientCA) > 0 {
				allErrs = append(allErrs, ValidateFile(provider.ClientCA, "provider.clientCA")...)
			}
			if len(provider.Headers) == 0 {
				allErrs = append(allErrs, fielderrors.NewFieldRequired("provider.headers"))
			}
			if identityProvider.Usage.UseAsChallenger {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("provider.useAsChallenger", identityProvider.Usage.UseAsChallenger, "request header providers cannot be used for challenges"))
			}
			if identityProvider.Usage.UseAsLogin {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("provider.useAsLogin", identityProvider.Usage.UseAsChallenger, "request header providers cannot be used for browser login"))
			}

		case (*api.BasicAuthPasswordIdentityProvider):
			allErrs = append(allErrs, ValidateRemoteConnectionInfo(provider.RemoteConnectionInfo).Prefix("provider")...)

		case (*api.HTPasswdPasswordIdentityProvider):
			allErrs = append(allErrs, ValidateFile(provider.File, "provider.file")...)

		case (*api.OAuthRedirectingIdentityProvider):
			if len(provider.ClientID) == 0 {
				allErrs = append(allErrs, fielderrors.NewFieldRequired("provider.clientID"))
			}
			if len(provider.ClientSecret) == 0 {
				allErrs = append(allErrs, fielderrors.NewFieldRequired("provider.clientSecret"))
			}
			if !api.IsOAuthProviderType(provider.Provider) {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("provider.provider", provider.Provider, fmt.Sprintf("%v is invalid in this context", identityProvider.Provider)))
			}
			if identityProvider.Usage.UseAsChallenger {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("provider.useAsChallenger", identityProvider.Usage.UseAsChallenger, "oauth providers cannot be used for challenges"))
			}
		}

	}

	return allErrs
}

func ValidateRemoteConnectionInfo(remoteConnectionInfo api.RemoteConnectionInfo) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(remoteConnectionInfo.URL) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("url"))
	}

	if len(remoteConnectionInfo.CA) > 0 {
		allErrs = append(allErrs, ValidateFile(remoteConnectionInfo.CA, "ca")...)
	}

	allErrs = append(allErrs, ValidateCertInfo(remoteConnectionInfo.ClientCert)...)

	return allErrs
}

func ValidateCertInfo(certInfo api.CertInfo) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(certInfo.CertFile) > 0 {
		if len(certInfo.KeyFile) == 0 {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("keyFile"))
		}

		allErrs = append(allErrs, ValidateFile(certInfo.CertFile, "certFile")...)
	}

	if len(certInfo.KeyFile) > 0 {
		if len(certInfo.CertFile) == 0 {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("certFile"))
		}

		allErrs = append(allErrs, ValidateFile(certInfo.KeyFile, "keyFile")...)
	}

	return allErrs
}

func ValidateGrantConfig(config api.GrantConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if !api.ValidGrantHandlerTypes.Has(string(config.Method)) {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("grantConfig.method", config.Method, fmt.Sprintf("must be one of: %v", api.ValidGrantHandlerTypes.List())))
	}

	return allErrs
}

func ValidateSessionConfig(config *api.SessionConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.SessionSecrets) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("sessionSecrets"))
	}
	if len(config.SessionName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("sessionName"))
	}

	return allErrs
}

func ValidateSpecifiedIP(ipString string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	ip := net.ParseIP(ipString)
	if ip == nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, ipString, "must be a valid IP"))
	} else if ip.IsUnspecified() {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, ipString, "cannot be an unspecified IP"))
	}

	return allErrs
}

func ValidateURL(urlString string, field string) (*url.URL, fielderrors.ValidationErrorList) {
	allErrs := fielderrors.ValidationErrorList{}

	urlObj, err := url.Parse(urlString)
	if err != nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, urlString, "must be a valid URL"))
		return nil, allErrs
	}
	if len(urlObj.Scheme) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, urlString, "must contain a scheme (e.g. http://)"))
	}
	if len(urlObj.Host) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, urlString, "must contain a host"))
	}
	return urlObj, allErrs
}

func ValidateAssetConfig(config *api.AssetConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateServingInfo(config.ServingInfo).Prefix("servingInfo")...)

	urlObj, urlErrs := ValidateURL(config.PublicURL, "publicURL")
	if len(urlErrs) > 0 {
		allErrs = append(allErrs, urlErrs...)
	}
	if urlObj != nil {
		if !strings.HasSuffix(urlObj.Path, "/") {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("publicURL", config.PublicURL, "must have a trailing slash in path"))
		}
	}

	return allErrs
}

func ValidateMasterConfig(config *api.MasterConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateServingInfo(config.ServingInfo).Prefix("servingInfo")...)

	if config.AssetConfig != nil {
		allErrs = append(allErrs, ValidateAssetConfig(config.AssetConfig).Prefix("assetConfig")...)
		colocated := config.AssetConfig.ServingInfo.BindAddress == config.ServingInfo.BindAddress
		if colocated {
			publicURL, _ := url.Parse(config.AssetConfig.PublicURL)
			if publicURL.Path == "/" {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("assetConfig.publicURL", config.AssetConfig.PublicURL, "path can not be / when colocated with master API"))
			}
		}

		if config.OAuthConfig != nil && config.OAuthConfig.AssetPublicURL != config.AssetConfig.PublicURL {
			allErrs = append(allErrs,
				fielderrors.NewFieldInvalid("assetConfig.publicURL", config.AssetConfig.PublicURL, "must match oauthConfig.assetPublicURL"),
				fielderrors.NewFieldInvalid("oauthConfig.assetPublicURL", config.OAuthConfig.AssetPublicURL, "must match assetConfig.publicURL"),
			)
		}

		// TODO warn when the CORS list does not include the assetConfig.publicURL host:port
		// only warn cause they could handle CORS headers themselves in a proxy
	}

	if config.DNSConfig != nil {
		allErrs = append(allErrs, ValidateBindAddress(config.DNSConfig.BindAddress).Prefix("dnsConfig")...)
	}

	if config.KubernetesMasterConfig != nil {
		allErrs = append(allErrs, ValidateKubernetesMasterConfig(config.KubernetesMasterConfig).Prefix("kubernetesMasterConfig")...)
	}

	allErrs = append(allErrs, ValidatePolicyConfig(config.PolicyConfig).Prefix("policyConfig")...)
	allErrs = append(allErrs, ValidateOAuthConfig(config.OAuthConfig).Prefix("oauthConfig")...)

	allErrs = append(allErrs, ValidateKubeConfig(config.MasterClients.DeployerKubeConfig, "deployerKubeConfig").Prefix("masterClients")...)
	allErrs = append(allErrs, ValidateKubeConfig(config.MasterClients.OpenShiftLoopbackKubeConfig, "openShiftLoopbackKubeConfig").Prefix("masterClients")...)
	allErrs = append(allErrs, ValidateKubeConfig(config.MasterClients.KubernetesKubeConfig, "kubernetesKubeConfig").Prefix("masterClients")...)

	return allErrs
}

func ValidatePolicyConfig(config api.PolicyConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateFile(config.BootstrapPolicyFile, "bootstrapPolicyFile")...)
	allErrs = append(allErrs, ValidateNamespace(config.MasterAuthorizationNamespace, "masterAuthorizationNamespace")...)
	allErrs = append(allErrs, ValidateNamespace(config.OpenShiftSharedResourcesNamespace, "openShiftSharedResourcesNamespace")...)

	return allErrs
}

func ValidateNamespace(namespace, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(namespace) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired(field))
	} else if ok, _ := kvalidation.ValidateNamespaceName(namespace, false); !ok {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, namespace, "must be a valid namespace"))
	}

	return allErrs
}

func ValidateNodeConfig(config *api.NodeConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.NodeName) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("nodeName"))
	}

	allErrs = append(allErrs, ValidateServingInfo(config.ServingInfo).Prefix("servingInfo")...)
	allErrs = append(allErrs, ValidateKubeConfig(config.MasterKubeConfig, "masterKubeConfig")...)

	if len(config.DNSIP) > 0 {
		allErrs = append(allErrs, ValidateSpecifiedIP(config.DNSIP, "dnsIP")...)
	}

	if len(config.NetworkContainerImage) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("networkContainerImage"))
	}

	return allErrs
}

func ValidateFile(path string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(path) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired(field))
	} else if _, err := os.Stat(path); err != nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, path, "could not read file"))
	}

	return allErrs
}

func ValidateAllInOneConfig(master *api.MasterConfig, node *api.NodeConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateMasterConfig(master).Prefix("masterConfig")...)

	allErrs = append(allErrs, ValidateNodeConfig(node).Prefix("nodeConfig")...)

	// Validation between the configs

	return allErrs
}
