package origin

import (
	"fmt"
	"github.com/golang/glog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/elazarl/go-bindata-assetfs"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/server"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	genericmux "k8s.io/apiserver/pkg/server/mux"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	kapi "k8s.io/kubernetes/pkg/api"
	kversion "k8s.io/kubernetes/pkg/version"

	"github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/assets"
	"github.com/openshift/origin/pkg/assets/java"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	oapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/pluginconfig"
	oauthutil "github.com/openshift/origin/pkg/oauth/util"
	"github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride"
	clusterresourceoverrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	oversion "github.com/openshift/origin/pkg/version"
	utilflag "k8s.io/apiserver/pkg/util/flag"
)

type AssetServerConfig struct {
	GenericConfig *genericapiserver.Config

	Options               oapi.AssetConfig
	LimitRequestOverrides *clusterresourceoverrideapi.ClusterResourceOverrideConfig

	PublicURL url.URL
}

// AssetServer serves non-API endpoints for openshift.
type AssetServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer

	PublicURL url.URL
}

type completedAssetServerConfig struct {
	*AssetServerConfig
}

// TODO this is taking a very large config for a small piece of it.  The information must be broken up at some point so that
// we can run this in a pod.  This is an indication of leaky abstraction because it spent too much time in openshift start
func NewAssetServerConfigFromMasterConfig(masterConfigOptions configapi.MasterConfig) (*AssetServerConfig, error) {
	assetConfig := masterConfigOptions.AssetConfig
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
	secureServingOptions := apiserveroptions.SecureServingOptions{}
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

	genericConfig := genericapiserver.NewConfig(kapi.Codecs)
	genericConfig.CorsAllowedOriginList = masterConfigOptions.CORSAllowedOrigins
	genericConfig.EnableDiscovery = false
	genericConfig.BuildHandlerChainFunc = buildHandlerChainForAssets(publicURL.Path)
	if err := secureServingOptions.ApplyTo(genericConfig); err != nil {
		return nil, err
	}
	genericConfig.SecureServingInfo.BindAddress = assetConfig.ServingInfo.BindAddress
	genericConfig.SecureServingInfo.BindNetwork = assetConfig.ServingInfo.BindNetwork
	genericConfig.SecureServingInfo.MinTLSVersion = crypto.TLSVersionOrDie(assetConfig.ServingInfo.MinTLSVersion)
	genericConfig.SecureServingInfo.CipherSuites = crypto.CipherSuitesOrDie(assetConfig.ServingInfo.CipherSuites)

	overrideConfig, err := getResourceOverrideConfig(masterConfigOptions)
	if err != nil {
		return nil, err
	}

	ret := &AssetServerConfig{
		GenericConfig:         genericConfig,
		Options:               *masterConfigOptions.AssetConfig,
		LimitRequestOverrides: overrideConfig,
		PublicURL:             *publicURL,
	}

	return ret, nil
}

// getResourceOverrideConfig looks in two potential places where ClusterResourceOverrideConfig can be specified
func getResourceOverrideConfig(masterConfigOptions configapi.MasterConfig) (*clusterresourceoverrideapi.ClusterResourceOverrideConfig, error) {
	overrideConfig, err := checkForOverrideConfig(masterConfigOptions.AdmissionConfig)
	if err != nil {
		return nil, err
	}
	if overrideConfig != nil {
		return overrideConfig, nil
	}
	if masterConfigOptions.KubernetesMasterConfig == nil { // external kube gets you a nil pointer here
		return nil, nil
	}
	overrideConfig, err = checkForOverrideConfig(masterConfigOptions.KubernetesMasterConfig.AdmissionConfig)
	if err != nil {
		return nil, err
	}
	return overrideConfig, nil
}

// checkForOverrideConfig looks for ClusterResourceOverrideConfig plugin cfg in the admission PluginConfig
func checkForOverrideConfig(ac configapi.AdmissionConfig) (*clusterresourceoverrideapi.ClusterResourceOverrideConfig, error) {
	overridePluginConfigFile, err := pluginconfig.GetPluginConfigFile(ac.PluginConfig, clusterresourceoverrideapi.PluginName, "")
	if err != nil {
		return nil, err
	}
	if overridePluginConfigFile == "" {
		return nil, nil
	}
	configFile, err := os.Open(overridePluginConfigFile)
	if err != nil {
		return nil, err
	}
	overrideConfig, err := clusterresourceoverride.ReadConfig(configFile)
	if err != nil {
		return nil, err
	}
	return overrideConfig, nil
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *AssetServerConfig) Complete() completedAssetServerConfig {
	c.GenericConfig.Complete()

	return completedAssetServerConfig{c}
}

// SkipComplete provides a way to construct a server instance without config completion.
func (c *AssetServerConfig) SkipComplete() completedAssetServerConfig {
	return completedAssetServerConfig{c}
}

func (c completedAssetServerConfig) New(delegationTarget genericapiserver.DelegationTarget) (*AssetServer, error) {
	genericServer, err := c.AssetServerConfig.GenericConfig.SkipComplete().New("openshift-non-api-routes", delegationTarget) // completion is done in Complete, no need for a second time
	if err != nil {
		return nil, err
	}

	s := &AssetServer{
		GenericAPIServer: genericServer,
		PublicURL:        c.PublicURL,
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
		handler = genericfilters.WithCORS(handler, c.CorsAllowedOriginList, nil, nil, nil, "true")
		handler = genericfilters.WithTimeoutForNonLongRunningRequests(handler, c.RequestContextMapper, c.LongRunningFunc)
		handler = genericapifilters.WithRequestInfo(handler, genericapiserver.NewRequestInfoResolver(c), c.RequestContextMapper)
		handler = apirequest.WithRequestContext(handler, c.RequestContextMapper)
		handler = genericfilters.WithPanicRecovery(handler)
		return handler
	}
}

func (c completedAssetServerConfig) addAssets(serverMux *genericmux.PathRecorderMux) error {
	assetHandler, err := c.buildAssetHandler()
	if err != nil {
		return err
	}

	serverMux.UnlistedHandlePrefix(c.PublicURL.Path, http.StripPrefix(c.PublicURL.Path, assetHandler))
	serverMux.UnlistedHandle(c.PublicURL.Path[0:len(c.PublicURL.Path)-1], http.RedirectHandler(c.PublicURL.Path, http.StatusMovedPermanently))
	return nil
}

func (c completedAssetServerConfig) addExtensionScripts(serverMux *genericmux.PathRecorderMux) error {
	// Extension scripts
	extScriptsPath := path.Join(c.PublicURL.Path, "scripts/extensions.js")
	extScriptsHandler, err := assets.ExtensionScriptsHandler(c.Options.ExtensionScripts, c.Options.ExtensionDevelopment)
	if err != nil {
		return err
	}
	extScriptsHandler = assets.SecurityHeadersHandler(extScriptsHandler)
	serverMux.UnlistedHandle(extScriptsPath, assets.GzipHandler(extScriptsHandler))
	return nil
}

func (c completedAssetServerConfig) addExtensionStyleSheets(serverMux *genericmux.PathRecorderMux) error {
	// Extension stylesheets
	extStylesheetsPath := path.Join(c.PublicURL.Path, "styles/extensions.css")
	extStylesheetsHandler, err := assets.ExtensionStylesheetsHandler(c.Options.ExtensionStylesheets, c.Options.ExtensionDevelopment)
	if err != nil {
		return err
	}
	extStylesheetsHandler = assets.SecurityHeadersHandler(extStylesheetsHandler)
	serverMux.UnlistedHandle(extStylesheetsPath, assets.GzipHandler(extStylesheetsHandler))
	return nil
}

func (c completedAssetServerConfig) addExtensionFiles(serverMux *genericmux.PathRecorderMux) {
	// Extension files
	for _, extConfig := range c.Options.Extensions {
		extBasePath := path.Join(c.PublicURL.Path, "extensions", extConfig.Name)
		extPath := extBasePath + "/"
		extHandler := assets.AssetExtensionHandler(extConfig.SourceDirectory, extPath, extConfig.HTML5Mode)
		serverMux.UnlistedHandlePrefix(extPath, http.StripPrefix(extBasePath, extHandler))
		serverMux.UnlistedHandle(extBasePath, http.RedirectHandler(extPath, http.StatusMovedPermanently))
	}
}

func (c *completedAssetServerConfig) addWebConsoleConfig(serverMux *genericmux.PathRecorderMux) error {
	masterURL, err := url.Parse(c.Options.MasterPublicURL)
	if err != nil {
		return err
	}

	originResources := sets.NewString()
	k8sResources := sets.NewString()

	versions := []schema.GroupVersion{}
	versions = append(versions, kapi.Registry.GroupOrDie(api.GroupName).GroupVersions...)
	versions = append(versions, kapi.Registry.GroupOrDie(kapi.GroupName).GroupVersions...)
	deadOriginVersions := sets.NewString(configapi.DeadOpenShiftAPILevels...)
	deadKubernetesVersions := sets.NewString(configapi.DeadKubernetesAPILevels...)
	for _, version := range versions {
		for kind := range kapi.Scheme.KnownTypes(version) {
			if strings.HasSuffix(kind, "List") {
				continue
			}
			resource, _ := meta.UnsafeGuessKindToResource(version.WithKind(kind))
			if latest.OriginKind(version.WithKind(kind)) {
				if !deadOriginVersions.Has(version.String()) {
					originResources.Insert(resource.Resource)
				}
			} else {
				if !deadKubernetesVersions.Has(version.String()) {
					k8sResources.Insert(resource.Resource)
				}
			}
		}
	}

	commonResources := sets.NewString()
	for _, r := range originResources.List() {
		if k8sResources.Has(r) {
			commonResources.Insert(r)
		}
	}
	if commonResources.Len() > 0 {
		return fmt.Errorf("Resources for kubernetes and origin types intersect: %v", commonResources.List())
	}

	// Generated web console config and server version
	config := assets.WebConsoleConfig{
		APIGroupAddr:          masterURL.Host,
		APIGroupPrefix:        server.APIGroupPrefix,
		MasterAddr:            masterURL.Host,
		MasterPrefix:          api.Prefix,
		MasterResources:       originResources.List(),
		KubernetesAddr:        masterURL.Host,
		KubernetesPrefix:      server.DefaultLegacyAPIPrefix,
		KubernetesResources:   k8sResources.List(),
		OAuthAuthorizeURI:     oauthutil.OpenShiftOAuthAuthorizeURL(masterURL.String()),
		OAuthTokenURI:         oauthutil.OpenShiftOAuthTokenURL(masterURL.String()),
		OAuthRedirectBase:     c.Options.PublicURL,
		OAuthClientID:         OpenShiftWebConsoleClientID,
		LogoutURI:             c.Options.LogoutURL,
		LoggingURL:            c.Options.LoggingPublicURL,
		MetricsURL:            c.Options.MetricsPublicURL,
		LimitRequestOverrides: c.LimitRequestOverrides,
	}
	kVersionInfo := kversion.Get()
	oVersionInfo := oversion.Get()
	versionInfo := assets.WebConsoleVersion{
		KubernetesVersion: kVersionInfo.GitVersion,
		OpenShiftVersion:  oVersionInfo.GitVersion,
	}

	extensionProps := assets.WebConsoleExtensionProperties{
		ExtensionProperties: extensionPropertyArray(c.Options.ExtensionProperties),
	}
	configPath := path.Join(c.PublicURL.Path, "config.js")
	configHandler, err := assets.GeneratedConfigHandler(config, versionInfo, extensionProps)
	configHandler = assets.SecurityHeadersHandler(configHandler)
	if err != nil {
		return err
	}
	serverMux.UnlistedHandle(configPath, assets.GzipHandler(configHandler))

	return nil
}

func (c completedAssetServerConfig) buildAssetHandler() (http.Handler, error) {
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
	handler, err = assets.HTML5ModeHandler(c.PublicURL.Path, subcontextMap, handler, assetFunc)
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
	glog.Infof("Web console available at %s", assetServer.PublicURL)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	return cmdutil.WaitForSuccessfulDial(true, assetServer.GenericAPIServer.SecureServingInfo.BindNetwork, assetServer.GenericAPIServer.SecureServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)
}
