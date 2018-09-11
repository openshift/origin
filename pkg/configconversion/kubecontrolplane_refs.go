package configconversion

import (
	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
)

func GetKubeAPIServerConfigFileReferences(config *kubecontrolplanev1.KubeAPIServerConfig) []*string {
	if config == nil {
		return []*string{}
	}

	refs := []*string{}

	refs = append(refs, helpers.GetGenericAPIServerConfigFileReferences(&config.GenericAPIServerConfig)...)
	refs = append(refs, GetKubeletConnectionInfoFileReferences(&config.KubeletClientInfo)...)

	if config.OAuthConfig != nil {
		refs = append(refs, GetOAuthConfigFileReferences(config.OAuthConfig)...)
	}

	refs = append(refs, &config.AggregatorConfig.ProxyClientInfo.CertFile)
	refs = append(refs, &config.AggregatorConfig.ProxyClientInfo.KeyFile)

	if config.AuthConfig.RequestHeader != nil {
		refs = append(refs, &config.AuthConfig.RequestHeader.ClientCA)
	}
	for k := range config.AuthConfig.WebhookTokenAuthenticators {
		refs = append(refs, &config.AuthConfig.WebhookTokenAuthenticators[k].ConfigFile)
	}
	if len(config.AuthConfig.OAuthMetadataFile) > 0 {
		refs = append(refs, &config.AuthConfig.OAuthMetadataFile)
	}

	refs = append(refs, &config.AggregatorConfig.ProxyClientInfo.CertFile)
	refs = append(refs, &config.AggregatorConfig.ProxyClientInfo.KeyFile)

	for i := range config.ServiceAccountPublicKeyFiles {
		refs = append(refs, &config.ServiceAccountPublicKeyFiles[i])
	}

	return refs
}

func GetKubeletConnectionInfoFileReferences(config *kubecontrolplanev1.KubeletConnectionInfo) []*string {
	if config == nil {
		return []*string{}
	}

	refs := []*string{}
	refs = append(refs, helpers.GetCertFileReferences(&config.CertInfo)...)
	refs = append(refs, &config.CA)
	return refs
}
