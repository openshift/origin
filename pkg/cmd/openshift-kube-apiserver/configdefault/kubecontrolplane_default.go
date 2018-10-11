package configdefault

import (
	"io/ioutil"
	"os"
	"path"

	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	"github.com/openshift/library-go/pkg/config/configdefaults"
)

func SetRecommendedKubeAPIServerConfigDefaults(config *kubecontrolplanev1.KubeAPIServerConfig) {
	configdefaults.DefaultString(&config.GenericAPIServerConfig.StorageConfig.StoragePrefix, "kubernetes.io")
	configdefaults.DefaultString(&config.GenericAPIServerConfig.ServingInfo.BindAddress, "0.0.0.0:6443")

	configdefaults.SetRecommendedGenericAPIServerConfigDefaults(&config.GenericAPIServerConfig)
	SetRecommendedMasterAuthConfigDefaults(&config.AuthConfig)
	SetRecommendedAggregatorConfigDefaults(&config.AggregatorConfig)
	SetRecommendedKubeletConnectionInfoDefaults(&config.KubeletClientInfo)

	configdefaults.DefaultString(&config.ServicesSubnet, "10.0.0.0/24")
	configdefaults.DefaultString(&config.ServicesNodePortRange, "30000-32767")

	if len(config.ServiceAccountPublicKeyFiles) == 0 {
		contents, err := ioutil.ReadDir("/var/run/configmaps/sa-token-signing-certs")
		switch {
		case os.IsNotExist(err) || os.IsPermission(err):
		case err != nil:
			panic(err) // some weird, unexpected error
		default:
			for _, content := range contents {
				if !content.Mode().IsRegular() {
					continue
				}
				config.ServiceAccountPublicKeyFiles = append(config.ServiceAccountPublicKeyFiles, path.Join("/var/run/configmaps/sa-token-signing-certs", content.Name()))
			}
		}
	}

	// after the aggregator defaults are set, we can default the auth config values
	// TODO this indicates that we're set two different things to the same value
	if config.AuthConfig.RequestHeader == nil {
		config.AuthConfig.RequestHeader = &kubecontrolplanev1.RequestHeaderAuthenticationOptions{}
		configdefaults.DefaultStringSlice(&config.AuthConfig.RequestHeader.ClientCommonNames, []string{"system:openshift-aggregator"})
		configdefaults.DefaultString(&config.AuthConfig.RequestHeader.ClientCA, "/var/run/configmaps/aggregator-client-ca/ca-bundle.crt")
		configdefaults.DefaultStringSlice(&config.AuthConfig.RequestHeader.UsernameHeaders, []string{"X-Remote-User"})
		configdefaults.DefaultStringSlice(&config.AuthConfig.RequestHeader.GroupHeaders, []string{"X-Remote-Group"})
		configdefaults.DefaultStringSlice(&config.AuthConfig.RequestHeader.ExtraHeaderPrefixes, []string{"X-Remote-Extra-"})
	}

	// Set defaults Cache TTLs for external Webhook Token Reviewers
	for i := range config.AuthConfig.WebhookTokenAuthenticators {
		if len(config.AuthConfig.WebhookTokenAuthenticators[i].CacheTTL) == 0 {
			config.AuthConfig.WebhookTokenAuthenticators[i].CacheTTL = "2m"
		}
	}

	if config.OAuthConfig != nil {
		for i := range config.OAuthConfig.IdentityProviders {
			// By default, only let one identity provider authenticate a particular user
			// If multiple identity providers collide, the second one in will fail to auth
			// The admin can set this to "add" if they want to allow new identities to join existing users
			configdefaults.DefaultString(&config.OAuthConfig.IdentityProviders[i].MappingMethod, "claim")
		}
	}
}

func SetRecommendedMasterAuthConfigDefaults(config *kubecontrolplanev1.MasterAuthConfig) {
}

func SetRecommendedAggregatorConfigDefaults(config *kubecontrolplanev1.AggregatorConfig) {
	configdefaults.DefaultString(&config.ProxyClientInfo.KeyFile, "/var/run/secrets/aggregator-client/tls.key")
	configdefaults.DefaultString(&config.ProxyClientInfo.CertFile, "/var/run/secrets/aggregator-client/tls.crt")
}

func SetRecommendedKubeletConnectionInfoDefaults(config *kubecontrolplanev1.KubeletConnectionInfo) {
	if config.Port == 0 {
		config.Port = 10250
	}
	configdefaults.DefaultString(&config.CertInfo.KeyFile, "/var/run/secrets/kubelet-client/tls.key")
	configdefaults.DefaultString(&config.CertInfo.CertFile, "/var/run/secrets/kubelet-client/tls.crt")
	configdefaults.DefaultString(&config.CA, "/var/run/configmaps/kubelet-serving-ca/ca-bundle.crt")
}
