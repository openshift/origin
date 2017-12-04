package apiserver

import (
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/elazarl/go-bindata-assetfs"

	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/features"
	"k8s.io/apiserver/pkg/server"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	genericmux "k8s.io/apiserver/pkg/server/mux"
	genericapiserveroptions "k8s.io/apiserver/pkg/server/options"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	utilflag "k8s.io/apiserver/pkg/util/flag"
	kversion "k8s.io/kubernetes/pkg/version"

	"github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/assets"
	"github.com/openshift/origin/pkg/assets/java"
	oapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	oauthutil "github.com/openshift/origin/pkg/oauth/util"
	"github.com/openshift/origin/pkg/util/httprequest"
	oversion "github.com/openshift/origin/pkg/version"
)

const (
	OpenShiftWebConsoleClientID = "openshift-web-console"
)

type ExtraConfig struct {
	Options   oapi.AssetConfig
	PublicURL url.URL
}

type AssetServerConfig struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// AssetServer serves non-API endpoints for openshift.
type AssetServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer

	PublicURL url.URL
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

type CompletedConfig struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*completedConfig
}

func NewAssetServerConfig(assetConfig oapi.AssetConfig) (*AssetServerConfig, error) {
	publicURL, err := url.Parse(assetConfig.PublicURL)
	if err != nil {
		glog.Fatal(err)
	}
	_, portString, err := net.SplitHostPort(assetConfig.ServingInfo.BindAddress)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, err
	}
	secureServingOptions := genericapiserveroptions.SecureServingOptions{}
	secureServingOptions.BindPort = port
	secureServingOptions.ServerCert.CertKey.CertFile = assetConfig.ServingInfo.ServerCert.CertFile
	secureServingOptions.ServerCert.CertKey.KeyFile = assetConfig.ServingInfo.ServerCert.KeyFile
	for _, nc := range assetConfig.ServingInfo.NamedCertificates {
		sniCert := utilflag.NamedCertKey{
			CertFile: nc.CertFile,
			KeyFile:  nc.KeyFile,
			Names:    nc.Names,
		}
		secureServingOptions.SNICertKeys = append(secureServingOptions.SNICertKeys, sniCert)
	}

	genericConfig := genericapiserver.NewConfig(legacyscheme.Codecs)
	genericConfig.EnableDiscovery = false
	genericConfig.BuildHandlerChainFunc = buildHandlerChainForAssets(publicURL.Path)
	if err := secureServingOptions.ApplyTo(genericConfig); err != nil {
		return nil, err
	}
	genericConfig.SecureServingInfo.BindAddress = assetConfig.ServingInfo.BindAddress
	genericConfig.SecureServingInfo.BindNetwork = assetConfig.ServingInfo.BindNetwork
	genericConfig.SecureServingInfo.MinTLSVersion = crypto.TLSVersionOrDie(assetConfig.ServingInfo.MinTLSVersion)
	genericConfig.SecureServingInfo.CipherSuites = crypto.CipherSuitesOrDie(assetConfig.ServingInfo.CipherSuites)

	return &AssetServerConfig{
		GenericConfig: &genericapiserver.RecommendedConfig{Config: *genericConfig},
		ExtraConfig: ExtraConfig{
			Options:   assetConfig,
			PublicURL: *publicURL,
		},
	}, nil
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *AssetServerConfig) Complete() completedConfig {
	cfg := completedConfig{
		c.GenericConfig.Complete(),
		&c.ExtraConfig,
	}

	return cfg
}

func (c completedConfig) New(delegationTarget genericapiserver.DelegationTarget) (*AssetServer, error) {
	genericServer, err := c.GenericConfig.New("openshift-non-api-routes", delegationTarget)
	if err != nil {
		return nil, err
	}

	s := &AssetServer{
		GenericAPIServer: genericServer,
		PublicURL:        c.ExtraConfig.PublicURL,
	}

	if err := c.addAssets(s.GenericAPIServer.Handler.NonGoRestfulMux); err != nil {
		return nil, err
	}
	if err := c.addExtensionScripts(s.GenericAPIServer.Handler.NonGoRestfulMux); err != nil {
		return nil, err
	}
	if err := c.addExtensionStyleSheets(s.GenericAPIServer.Handler.NonGoRestfulMux); err != nil {
		return nil, err
	}
	c.addExtensionFiles(s.GenericAPIServer.Handler.NonGoRestfulMux)
	if err := c.addWebConsoleConfig(s.GenericAPIServer.Handler.NonGoRestfulMux); err != nil {
		return nil, err
	}

	return s, nil
}

// buildHandlerChainForAssets is the handling chain used to protect the asset server.  With no secret information to protect
// the chain is very short.
func buildHandlerChainForAssets(consoleRedirectPath string) func(startingHandler http.Handler, c *genericapiserver.Config) http.Handler {
	return func(startingHandler http.Handler, c *genericapiserver.Config) http.Handler {
		handler := WithAssetServerRedirect(startingHandler, consoleRedirectPath)
		handler = genericfilters.WithMaxInFlightLimit(handler, c.MaxRequestsInFlight, c.MaxMutatingRequestsInFlight, c.RequestContextMapper, c.LongRunningFunc)
		if utilfeature.DefaultFeatureGate.Enabled(features.AdvancedAuditing) {
			handler = genericapifilters.WithAudit(handler, c.RequestContextMapper, c.AuditBackend, c.AuditPolicyChecker, c.LongRunningFunc)
		}
		handler = genericfilters.WithTimeoutForNonLongRunningRequests(handler, c.RequestContextMapper, c.LongRunningFunc, c.RequestTimeout)
		handler = genericapifilters.WithRequestInfo(handler, genericapiserver.NewRequestInfoResolver(c), c.RequestContextMapper)
		handler = apirequest.WithRequestContext(handler, c.RequestContextMapper)
		handler = genericfilters.WithPanicRecovery(handler)

		return handler
	}
}

func (c completedConfig) addAssets(serverMux *genericmux.PathRecorderMux) error {
	assetHandler, err := c.buildAssetHandler()
	if err != nil {
		return err
	}

	serverMux.UnlistedHandlePrefix(c.ExtraConfig.PublicURL.Path, http.StripPrefix(c.ExtraConfig.PublicURL.Path, assetHandler))
	serverMux.UnlistedHandle(c.ExtraConfig.PublicURL.Path[0:len(c.ExtraConfig.PublicURL.Path)-1], http.RedirectHandler(c.ExtraConfig.PublicURL.Path, http.StatusMovedPermanently))
	return nil
}

func (c completedConfig) addExtensionScripts(serverMux *genericmux.PathRecorderMux) error {
	// Extension scripts
	extScriptsPath := path.Join(c.ExtraConfig.PublicURL.Path, "scripts/extensions.js")
	extScriptsHandler, err := assets.ExtensionScriptsHandler(c.ExtraConfig.Options.ExtensionScripts, c.ExtraConfig.Options.ExtensionDevelopment)
	if err != nil {
		return err
	}
	extScriptsHandler = assets.SecurityHeadersHandler(extScriptsHandler)
	serverMux.UnlistedHandle(extScriptsPath, assets.GzipHandler(extScriptsHandler))
	return nil
}

func (c completedConfig) addExtensionStyleSheets(serverMux *genericmux.PathRecorderMux) error {
	// Extension stylesheets
	extStylesheetsPath := path.Join(c.ExtraConfig.PublicURL.Path, "styles/extensions.css")
	extStylesheetsHandler, err := assets.ExtensionStylesheetsHandler(c.ExtraConfig.Options.ExtensionStylesheets, c.ExtraConfig.Options.ExtensionDevelopment)
	if err != nil {
		return err
	}
	extStylesheetsHandler = assets.SecurityHeadersHandler(extStylesheetsHandler)
	serverMux.UnlistedHandle(extStylesheetsPath, assets.GzipHandler(extStylesheetsHandler))
	return nil
}

func (c completedConfig) addExtensionFiles(serverMux *genericmux.PathRecorderMux) {
	// Extension files
	for _, extConfig := range c.ExtraConfig.Options.Extensions {
		extBasePath := path.Join(c.ExtraConfig.PublicURL.Path, "extensions", extConfig.Name)
		extPath := extBasePath + "/"
		extHandler := assets.AssetExtensionHandler(extConfig.SourceDirectory, extPath, extConfig.HTML5Mode)
		serverMux.UnlistedHandlePrefix(extPath, http.StripPrefix(extBasePath, extHandler))
		serverMux.UnlistedHandle(extBasePath, http.RedirectHandler(extPath, http.StatusMovedPermanently))
	}
}

func (c *completedConfig) addWebConsoleConfig(serverMux *genericmux.PathRecorderMux) error {
	masterURL, err := url.Parse(c.ExtraConfig.Options.MasterPublicURL)
	if err != nil {
		return err
	}

	// Generated web console config and server version
	config := assets.WebConsoleConfig{
		APIGroupAddr:      masterURL.Host,
		APIGroupPrefix:    server.APIGroupPrefix,
		MasterAddr:        masterURL.Host,
		MasterPrefix:      api.Prefix,
		KubernetesAddr:    masterURL.Host,
		KubernetesPrefix:  server.DefaultLegacyAPIPrefix,
		OAuthAuthorizeURI: oauthutil.OpenShiftOAuthAuthorizeURL(masterURL.String()),
		OAuthTokenURI:     oauthutil.OpenShiftOAuthTokenURL(masterURL.String()),
		OAuthRedirectBase: c.ExtraConfig.Options.PublicURL,
		OAuthClientID:     OpenShiftWebConsoleClientID,
		LogoutURI:         c.ExtraConfig.Options.LogoutURL,
		LoggingURL:        c.ExtraConfig.Options.LoggingPublicURL,
		MetricsURL:        c.ExtraConfig.Options.MetricsPublicURL,
	}
	kVersionInfo := kversion.Get()
	oVersionInfo := oversion.Get()
	versionInfo := assets.WebConsoleVersion{
		KubernetesVersion: kVersionInfo.GitVersion,
		OpenShiftVersion:  oVersionInfo.GitVersion,
	}

	extensionProps := assets.WebConsoleExtensionProperties{
		ExtensionProperties: extensionPropertyArray(c.ExtraConfig.Options.ExtensionProperties),
	}
	configPath := path.Join(c.ExtraConfig.PublicURL.Path, "config.js")
	configHandler, err := assets.GeneratedConfigHandler(config, versionInfo, extensionProps)
	configHandler = assets.SecurityHeadersHandler(configHandler)
	if err != nil {
		return err
	}
	serverMux.UnlistedHandle(configPath, assets.GzipHandler(configHandler))

	return nil
}

func (c completedConfig) buildAssetHandler() (http.Handler, error) {
	assets.RegisterMimeTypes()

	assetFunc := assets.JoinAssetFuncs(assets.Asset, java.Asset)
	assetDirFunc := assets.JoinAssetDirFuncs(assets.AssetDir, java.AssetDir)

	handler := http.FileServer(&assetfs.AssetFS{Asset: assetFunc, AssetDir: assetDirFunc, Prefix: ""})

	// Map of context roots (no leading or trailing slash) to the asset path to serve for requests to a missing asset
	subcontextMap := map[string]string{
		"":     "index.html",
		"java": "java/index.html",
	}

	var err error
	handler, err = assets.HTML5ModeHandler(c.ExtraConfig.PublicURL.Path, subcontextMap, handler, assetFunc)
	if err != nil {
		return nil, err
	}

	// Cache control should happen after all Vary headers are added, but before
	// any asset related routing (HTML5ModeHandler and FileServer)
	handler = assets.CacheControlHandler(oversion.Get().GitCommit, handler)

	handler = assets.SecurityHeadersHandler(handler)

	// Gzip first so that inner handlers can react to the addition of the Vary header
	handler = assets.GzipHandler(handler)

	return handler, nil
}

// Have to convert to arrays because go templates are limited and we need to be able to know
// if we are on the last index for trailing commas in JSON
func extensionPropertyArray(extensionProperties map[string]string) []assets.WebConsoleExtensionProperty {
	extensionPropsArray := []assets.WebConsoleExtensionProperty{}
	for key, value := range extensionProperties {
		extensionPropsArray = append(extensionPropsArray, assets.WebConsoleExtensionProperty{
			Key:   key,
			Value: value,
		})
	}
	return extensionPropsArray
}

// Run starts an http server for the static assets listening on the configured
// bind address
func RunAssetServer(assetServer *AssetServer, stopCh <-chan struct{}) error {
	go assetServer.GenericAPIServer.PrepareRun().Run(stopCh)

	glog.Infof("Web console listening at https://%s", assetServer.GenericAPIServer.SecureServingInfo.BindAddress)
	glog.Infof("Web console available at %s", assetServer.PublicURL.String())

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	return cmdutil.WaitForSuccessfulDial(true, assetServer.GenericAPIServer.SecureServingInfo.BindNetwork, assetServer.GenericAPIServer.SecureServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
}

// If we know the location of the asset server, redirect to it when / is requested
// and the Accept header supports text/html
func WithAssetServerRedirect(handler http.Handler, assetPublicURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			if httprequest.PrefersHTML(req) {
				http.Redirect(w, req, assetPublicURL, http.StatusFound)
			}
		}
		// Dispatch to the next handler
		handler.ServeHTTP(w, req)
	})
}
