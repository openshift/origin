package origin

import (
	"net"
	"strconv"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	oauthapiserver "github.com/openshift/origin/pkg/oauth/apiserver"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	utilflag "k8s.io/apiserver/pkg/util/flag"
)

// TODO this is taking a very large config for a small piece of it.  The information must be broken up at some point so that
// we can run this in a pod.  This is an indication of leaky abstraction because it spent too much time in openshift start
func NewOAuthServerConfigFromMasterConfig(masterConfig *MasterConfig) (*oauthapiserver.OAuthServerConfig, error) {
	options := masterConfig.Options
	servingConfig := options.ServingInfo
	oauthConfig := masterConfig.Options.OAuthConfig

	oauthServerConfig, err := oauthapiserver.NewOAuthServerConfig(*oauthConfig, &masterConfig.PrivilegedLoopbackClientConfig)
	if err != nil {
		return nil, err
	}

	oauthServerConfig.GenericConfig.CorsAllowedOriginList = options.CORSAllowedOrigins

	// TODO pull this out into a function
	_, portString, err := net.SplitHostPort(servingConfig.BindAddress)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, err
	}
	secureServingOptions := apiserveroptions.SecureServingOptions{}
	secureServingOptions.BindPort = port
	secureServingOptions.ServerCert.CertKey.CertFile = servingConfig.ServerCert.CertFile
	secureServingOptions.ServerCert.CertKey.KeyFile = servingConfig.ServerCert.KeyFile
	for _, nc := range servingConfig.NamedCertificates {
		sniCert := utilflag.NamedCertKey{
			CertFile: nc.CertFile,
			KeyFile:  nc.KeyFile,
			Names:    nc.Names,
		}
		secureServingOptions.SNICertKeys = append(secureServingOptions.SNICertKeys, sniCert)
	}
	if err := secureServingOptions.ApplyTo(oauthServerConfig.GenericConfig); err != nil {
		return nil, err
	}
	oauthServerConfig.GenericConfig.SecureServingInfo.BindAddress = servingConfig.BindAddress
	oauthServerConfig.GenericConfig.SecureServingInfo.BindNetwork = servingConfig.BindNetwork
	oauthServerConfig.GenericConfig.SecureServingInfo.MinTLSVersion = crypto.TLSVersionOrDie(servingConfig.MinTLSVersion)
	oauthServerConfig.GenericConfig.SecureServingInfo.CipherSuites = crypto.CipherSuitesOrDie(servingConfig.CipherSuites)

	// TODO pass a privileged client config through during construction.  It is NOT a loopback client.
	oauthServerConfig.OpenShiftClient = masterConfig.PrivilegedLoopbackOpenShiftClient
	oauthServerConfig.KubeClient = masterConfig.PrivilegedLoopbackKubernetesClientsetInternal

	// Build the list of valid redirect_uri prefixes for a login using the openshift-web-console client to redirect to
	if !options.DisabledFeatures.Has(configapi.FeatureWebConsole) {
		oauthServerConfig.AssetPublicAddresses = []string{oauthConfig.AssetPublicURL}
	}

	return oauthServerConfig, nil
}
