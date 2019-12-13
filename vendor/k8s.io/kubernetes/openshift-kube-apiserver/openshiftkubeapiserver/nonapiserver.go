package openshiftkubeapiserver

import (
	"net/http"

	"k8s.io/klog"

	genericapiserver "k8s.io/apiserver/pkg/server"
	genericmux "k8s.io/apiserver/pkg/server/mux"
	"k8s.io/client-go/informers"

	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	osinv1 "github.com/openshift/api/osin/v1"
)

func NewOpenshiftNonAPIConfig(generiConfig *genericapiserver.Config, kubeInformers informers.SharedInformerFactory, oauthConfig *osinv1.OAuthConfig, authConfig kubecontrolplanev1.MasterAuthConfig) (*OpenshiftNonAPIConfig, error) {
	var err error
	ret := &OpenshiftNonAPIConfig{
		GenericConfig: &genericapiserver.RecommendedConfig{
			Config:                *generiConfig,
			SharedInformerFactory: kubeInformers,
		},
	}
	ret.ExtraConfig.OAuthMetadata, _, err = prepOauthMetadata(oauthConfig, authConfig.OAuthMetadataFile)
	if err != nil {
		// invalid metadata must not prevent the kube api server from starting
		klog.Errorf("Unable to initialize OAuth authorization server metadata: %v", err)
	}

	return ret, nil
}

type NonAPIExtraConfig struct {
	OAuthMetadata []byte
}

type OpenshiftNonAPIConfig struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   NonAPIExtraConfig
}

// OpenshiftNonAPIServer serves non-API endpoints for openshift.
type OpenshiftNonAPIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedOpenshiftNonAPIConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *NonAPIExtraConfig
}

type CompletedOpenshiftNonAPIConfig struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*completedOpenshiftNonAPIConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *OpenshiftNonAPIConfig) Complete() completedOpenshiftNonAPIConfig {
	cfg := completedOpenshiftNonAPIConfig{
		c.GenericConfig.Complete(),
		&c.ExtraConfig,
	}

	return cfg
}

func (c completedOpenshiftNonAPIConfig) New(delegationTarget genericapiserver.DelegationTarget) (*OpenshiftNonAPIServer, error) {
	genericServer, err := c.GenericConfig.New("openshift-non-api-routes", delegationTarget)
	if err != nil {
		return nil, err
	}

	s := &OpenshiftNonAPIServer{
		GenericAPIServer: genericServer,
	}

	// TODO move this up to the spot where we wire the oauth endpoint
	// Set up OAuth metadata only if we are configured to use OAuth
	if len(c.ExtraConfig.OAuthMetadata) > 0 {
		initOAuthAuthorizationServerMetadataRoute(s.GenericAPIServer.Handler.NonGoRestfulMux, c.ExtraConfig)
	}

	return s, nil
}

const (
	// Discovery endpoint for OAuth 2.0 Authorization Server Metadata
	// See IETF Draft:
	// https://tools.ietf.org/html/draft-ietf-oauth-discovery-04#section-2
	oauthMetadataEndpoint = "/.well-known/oauth-authorization-server"
)

// initOAuthAuthorizationServerMetadataRoute initializes an HTTP endpoint for OAuth 2.0 Authorization Server Metadata discovery
// https://tools.ietf.org/id/draft-ietf-oauth-discovery-04.html#rfc.section.2
// masterPublicURL should be internally and externally routable to allow all users to discover this information
func initOAuthAuthorizationServerMetadataRoute(mux *genericmux.PathRecorderMux, ExtraConfig *NonAPIExtraConfig) {
	mux.UnlistedHandleFunc(oauthMetadataEndpoint, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(ExtraConfig.OAuthMetadata)
	})
}
