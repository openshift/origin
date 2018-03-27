package origin

import (
	"net"
	"strconv"

	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	utilflag "k8s.io/apiserver/pkg/util/flag"

	routeclient "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/oauthserver/oauthserver"
)

// TODO this is taking a very large config for a small piece of it.  The information must be broken up at some point so that
// we can run this in a pod.  This is an indication of leaky abstraction because it spent too much time in openshift start
func NewOAuthServerConfigFromMasterConfig(masterConfig *MasterConfig, listener net.Listener) (*oauthserver.OAuthServerConfig, error) {
	options := masterConfig.Options
	servingConfig := options.ServingInfo
	oauthConfig := masterConfig.Options.OAuthConfig

	oauthServerConfig, err := oauthserver.NewOAuthServerConfig(*oauthConfig, &masterConfig.PrivilegedLoopbackClientConfig)
	if err != nil {
		return nil, err
	}

	oauthServerConfig.GenericConfig.CorsAllowedOriginList = options.CORSAllowedOrigins

	// TODO pull this out into a function
	host, portString, err := net.SplitHostPort(servingConfig.BindAddress)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, err
	}
	secureServingOptions := apiserveroptions.SecureServingOptions{}
	secureServingOptions.Listener = listener
	secureServingOptions.BindAddress = net.ParseIP(host)
	secureServingOptions.BindNetwork = servingConfig.BindNetwork
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
	if err := secureServingOptions.ApplyTo(&oauthServerConfig.GenericConfig.Config); err != nil {
		return nil, err
	}
	oauthServerConfig.GenericConfig.SecureServingInfo.MinTLSVersion = crypto.TLSVersionOrDie(servingConfig.MinTLSVersion)
	oauthServerConfig.GenericConfig.SecureServingInfo.CipherSuites = crypto.CipherSuitesOrDie(servingConfig.CipherSuites)

	routeClient, err := routeclient.NewForConfig(&masterConfig.PrivilegedLoopbackClientConfig)
	if err != nil {
		return nil, err
	}
	// TODO pass a privileged client config through during construction.  It is NOT a loopback client.
	oauthServerConfig.ExtraOAuthConfig.RouteClient = routeClient
	oauthServerConfig.ExtraOAuthConfig.KubeClient = masterConfig.PrivilegedLoopbackKubernetesClientsetExternal

	// Build the list of valid redirect_uri prefixes for a login using the openshift-web-console client to redirect to
	oauthServerConfig.ExtraOAuthConfig.AssetPublicAddresses = []string{oauthConfig.AssetPublicURL}

	return oauthServerConfig, nil
}
