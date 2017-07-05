package origin

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/elazarl/go-bindata-assetfs"
	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server"
	kapi "k8s.io/kubernetes/pkg/api"
	kversion "k8s.io/kubernetes/pkg/version"

	"github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/assets"
	"github.com/openshift/origin/pkg/assets/java"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	oauthutil "github.com/openshift/origin/pkg/oauth/util"
	oversion "github.com/openshift/origin/pkg/version"
)

// WithAssets decorates a handler by serving static assets for the subpath of
// the public URL and passing through all other requests to the given handler.
func (c *AssetConfig) WithAssets(handler http.Handler) (http.Handler, error) {
	if c == nil {
		return handler, nil
	}

	return c.addHandlers(handler)
}

// Run starts an http server for the static assets listening on the configured
// bind address
func (c *AssetConfig) Run() {
	mux, err := c.addHandlers(nil)
	if err != nil {
		glog.Fatal(err)
	}

	timeout := c.Options.ServingInfo.RequestTimeoutSeconds
	if timeout == -1 {
		timeout = 0
	}

	server := &http.Server{
		Addr:           c.Options.ServingInfo.BindAddress,
		Handler:        mux,
		ReadTimeout:    time.Duration(timeout) * time.Second,
		WriteTimeout:   time.Duration(timeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	isTLS := configapi.UseTLS(c.Options.ServingInfo.ServingInfo)

	go utilwait.Forever(func() {
		if isTLS {
			extraCerts, err := configapi.GetNamedCertificateMap(c.Options.ServingInfo.NamedCertificates)
			if err != nil {
				glog.Fatal(err)
			}
			server.TLSConfig = crypto.SecureTLSConfig(&tls.Config{
				// Set SNI certificate func
				GetCertificate: cmdutil.GetCertificateFunc(extraCerts),
				MinVersion:     crypto.TLSVersionOrDie(c.Options.ServingInfo.MinTLSVersion),
				CipherSuites:   crypto.CipherSuitesOrDie(c.Options.ServingInfo.CipherSuites),
			})
			glog.Infof("Web console listening at https://%s", c.Options.ServingInfo.BindAddress)
			glog.Fatal(cmdutil.ListenAndServeTLS(server, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.ServerCert.CertFile, c.Options.ServingInfo.ServerCert.KeyFile))
		} else {
			glog.Infof("Web console listening at http://%s", c.Options.ServingInfo.BindAddress)
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial(isTLS, c.Options.ServingInfo.BindNetwork, c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)

	glog.Infof("Web console available at %s", c.Options.PublicURL)
}

func (c *AssetConfig) buildAssetHandler() (http.Handler, error) {
	assets.RegisterMimeTypes()

	publicURL, err := url.Parse(c.Options.PublicURL)
	if err != nil {
		glog.Fatal(err)
	}

	assetFunc := assets.JoinAssetFuncs(assets.Asset, java.Asset)
	assetDirFunc := assets.JoinAssetDirFuncs(assets.AssetDir, java.AssetDir)

	handler := http.FileServer(&assetfs.AssetFS{Asset: assetFunc, AssetDir: assetDirFunc, Prefix: ""})

	// Map of context roots (no leading or trailing slash) to the asset path to serve for requests to a missing asset
	subcontextMap := map[string]string{
		"":     "index.html",
		"java": "java/index.html",
	}

	handler, err = assets.HTML5ModeHandler(publicURL.Path, subcontextMap, handler, assetFunc)
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

func (c *AssetConfig) addHandlers(handler http.Handler) (http.Handler, error) {
	publicURL, err := url.Parse(c.Options.PublicURL)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	if handler != nil {
		// colocated with other routes, so pass any unrecognized routes through to
		// handler
		mux.Handle("/", handler)
	} else {
		// standalone mode, so redirect any unrecognized routes to the console
		if publicURL.Path != "/" {
			mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
				http.Redirect(w, req, publicURL.Path, http.StatusFound)
			})
		}
	}

	assetHandler, err := c.buildAssetHandler()
	if err != nil {
		return nil, err
	}

	masterURL, err := url.Parse(c.Options.MasterPublicURL)
	if err != nil {
		return nil, err
	}

	// Web console assets
	mux.Handle(publicURL.Path, http.StripPrefix(publicURL.Path, assetHandler))

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
		return nil, fmt.Errorf("Resources for kubernetes and origin types intersect: %v", commonResources.List())
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
	configPath := path.Join(publicURL.Path, "config.js")
	configHandler, err := assets.GeneratedConfigHandler(config, versionInfo, extensionProps)
	configHandler = assets.SecurityHeadersHandler(configHandler)
	if err != nil {
		return nil, err
	}
	mux.Handle(configPath, assets.GzipHandler(configHandler))

	// Extension scripts
	extScriptsPath := path.Join(publicURL.Path, "scripts/extensions.js")
	extScriptsHandler, err := assets.ExtensionScriptsHandler(c.Options.ExtensionScripts, c.Options.ExtensionDevelopment)
	extScriptsHandler = assets.SecurityHeadersHandler(extScriptsHandler)
	if err != nil {
		return nil, err
	}
	mux.Handle(extScriptsPath, assets.GzipHandler(extScriptsHandler))

	// Extension stylesheets
	extStylesheetsPath := path.Join(publicURL.Path, "styles/extensions.css")
	extStylesheetsHandler, err := assets.ExtensionStylesheetsHandler(c.Options.ExtensionStylesheets, c.Options.ExtensionDevelopment)
	extStylesheetsHandler = assets.SecurityHeadersHandler(extStylesheetsHandler)
	if err != nil {
		return nil, err
	}
	mux.Handle(extStylesheetsPath, assets.GzipHandler(extStylesheetsHandler))

	// Extension files
	for _, extConfig := range c.Options.Extensions {
		extBasePath := path.Join(publicURL.Path, "extensions", extConfig.Name)
		extPath := extBasePath + "/"
		extHandler := assets.AssetExtensionHandler(extConfig.SourceDirectory, extPath, extConfig.HTML5Mode)
		mux.Handle(extPath, http.StripPrefix(extBasePath, extHandler))
	}

	return mux, nil
}
