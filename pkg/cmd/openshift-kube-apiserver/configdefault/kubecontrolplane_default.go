package configdefault

import (
	"io/ioutil"
	"os"
	"path"

	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
)

func SetRecommendedKubeAPIServerConfigDefaults(config *kubecontrolplanev1.KubeAPIServerConfig) {
	DefaultString(&config.GenericAPIServerConfig.StorageConfig.StoragePrefix, "kubernetes.io")

	SetRecommendedGenericAPIServerConfigDefaults(&config.GenericAPIServerConfig)
	SetRecommendedMasterAuthConfigDefaults(&config.AuthConfig)
	SetRecommendedAggregatorConfigDefaults(&config.AggregatorConfig)
	SetRecommendedKubeletConnectionInfoDefaults(&config.KubeletClientInfo)

	DefaultString(&config.ServicesSubnet, "10.0.0.0/24")
	DefaultString(&config.ServicesNodePortRange, "30000-32767")

	if len(config.ServiceAccountPublicKeyFiles) == 0 {
		contents, err := ioutil.ReadDir("/var/run/configmaps/sa-token-signing-certs")
		switch {
		case os.IsNotExist(err) || os.IsPermission(err):
		case err != nil:
			panic(err) // some weird, unexpected error
		default:
			for _, content := range contents {
				config.ServiceAccountPublicKeyFiles = append(config.ServiceAccountPublicKeyFiles, path.Join("/var/run/configmaps/sa-token-signing-certs", content.Name()))
			}
		}
	}

	// after the aggregator defaults are set, we can default the auth config values
	// TODO this indicates that we're set two different things to the same value
	if config.AuthConfig.RequestHeader == nil {
		config.AuthConfig.RequestHeader = &kubecontrolplanev1.RequestHeaderAuthenticationOptions{}
		DefaultStringSlice(&config.AuthConfig.RequestHeader.ClientCommonNames, []string{"system:openshift-aggregator"})
		DefaultString(&config.AuthConfig.RequestHeader.ClientCA, "/var/run/secrets/aggregator-client-ca/ca-bundle.crt")
		DefaultStringSlice(&config.AuthConfig.RequestHeader.UsernameHeaders, []string{"X-Remote-User"})
		DefaultStringSlice(&config.AuthConfig.RequestHeader.GroupHeaders, []string{"X-Remote-Group"})
		DefaultStringSlice(&config.AuthConfig.RequestHeader.ExtraHeaderPrefixes, []string{"X-Remote-Extra-"})
	}
}

func SetRecommendedMasterAuthConfigDefaults(config *kubecontrolplanev1.MasterAuthConfig) {
}

func SetRecommendedAggregatorConfigDefaults(config *kubecontrolplanev1.AggregatorConfig) {
	DefaultString(&config.ProxyClientInfo.KeyFile, "/var/run/secrets/aggregator-client/tls.key")
	DefaultString(&config.ProxyClientInfo.CertFile, "/var/run/secrets/aggregator-client/tls.crt")
}

func SetRecommendedKubeletConnectionInfoDefaults(config *kubecontrolplanev1.KubeletConnectionInfo) {
	if config.Port == 0 {
		config.Port = 10250
	}
	DefaultString(&config.CertInfo.KeyFile, "/var/run/secrets/kubelet-client/tls.key")
	DefaultString(&config.CertInfo.CertFile, "/var/run/secrets/kubelet-client/tls.crt")
	DefaultString(&config.CA, "/var/run/configmaps/kubelet-serving-ca/ca-bundle.crt")
}
