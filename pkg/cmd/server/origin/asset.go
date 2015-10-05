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
	"github.com/emicklei/go-restful"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/assets"
	"github.com/openshift/origin/pkg/assets/java"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/version"

	"k8s.io/kubernetes/pkg/api"
	klatest "k8s.io/kubernetes/pkg/api/latest"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"
)

// InstallAPI adds handlers for serving static assets into the provided mux,
// then returns an array of strings indicating what endpoints were started
// (these are format strings that will expect to be sent a single string value).
func (c *AssetConfig) InstallAPI(container *restful.Container) []string {
	publicURL, err := url.Parse(c.Options.PublicURL)
	if err != nil {
		glog.Fatal(err)
	}

	err = c.addHandlers(container.ServeMux)
	if err != nil {
		glog.Fatal(err)
	}

	return []string{fmt.Sprintf("Started Web Console %%s%s", publicURL.Path)}
}

// Run starts an http server for the static assets listening on the configured
// bind address
func (c *AssetConfig) Run() {
	publicURL, err := url.Parse(c.Options.PublicURL)
	if err != nil {
		glog.Fatal(err)
	}

	mux := http.NewServeMux()
	err = c.addHandlers(mux)
	if err != nil {
		glog.Fatal(err)
	}

	if publicURL.Path != "/" {
		mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, publicURL.Path, http.StatusFound)
		})
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

	go util.Forever(func() {
		if isTLS {
			server.TLSConfig = &tls.Config{
				// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
				MinVersion: tls.VersionTLS10,
			}
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
	handler = assets.CacheControlHandler(version.Get().GitCommit, handler)

	// Gzip first so that inner handlers can react to the addition of the Vary header
	handler = assets.GzipHandler(handler)

	return handler, nil
}

func (c *AssetConfig) addHandlers(mux *http.ServeMux) error {
	assetHandler, err := c.buildAssetHandler()
	if err != nil {
		return err
	}

	publicURL, err := url.Parse(c.Options.PublicURL)
	if err != nil {
		return err
	}

	masterURL, err := url.Parse(c.Options.MasterPublicURL)
	if err != nil {
		return err
	}

	// Web console assets
	mux.Handle(publicURL.Path, http.StripPrefix(publicURL.Path, assetHandler))

	originResources := sets.NewString()
	k8sResources := sets.NewString()

	versions := sets.NewString()
	versions.Insert(latest.Versions...)
	versions.Insert(klatest.Versions...)
	for _, version := range versions.List() {
		for kind, t := range api.Scheme.KnownTypes(version) {
			if strings.Contains(t.PkgPath(), "kubernetes/pkg/expapi") {
				continue
			}
			if strings.HasSuffix(kind, "List") {
				continue
			}
			resource, _ := meta.KindToResource(kind, false)
			if latest.OriginKind(kind, version) {
				originResources.Insert(resource)
			} else {
				k8sResources.Insert(resource)
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

	// Generated web console config
	config := assets.WebConsoleConfig{
		MasterAddr:          masterURL.Host,
		MasterPrefix:        OpenShiftAPIPrefix,
		MasterLegacyPrefix:  LegacyOpenShiftAPIPrefix,
		MasterResources:     originResources.List(),
		KubernetesAddr:      masterURL.Host,
		KubernetesPrefix:    KubernetesAPIPrefix,
		KubernetesResources: k8sResources.List(),
		OAuthAuthorizeURI:   OpenShiftOAuthAuthorizeURL(masterURL.String()),
		OAuthRedirectBase:   c.Options.PublicURL,
		OAuthClientID:       OpenShiftWebConsoleClientID,
		LogoutURI:           c.Options.LogoutURL,
	}
	configPath := path.Join(publicURL.Path, "config.js")
	configHandler, err := assets.GeneratedConfigHandler(config)
	if err != nil {
		return err
	}
	mux.Handle(configPath, assets.GzipHandler(configHandler))

	// Extension scripts
	extScriptsPath := path.Join(publicURL.Path, "scripts/extensions.js")
	extScriptsHandler, err := assets.ExtensionScriptsHandler(c.Options.ExtensionScripts, c.Options.ExtensionDevelopment)
	if err != nil {
		return err
	}
	mux.Handle(extScriptsPath, assets.GzipHandler(extScriptsHandler))

	// Extension stylesheets
	extStylesheetsPath := path.Join(publicURL.Path, "styles/extensions.css")
	extStylesheetsHandler, err := assets.ExtensionStylesheetsHandler(c.Options.ExtensionStylesheets, c.Options.ExtensionDevelopment)
	if err != nil {
		return err
	}
	mux.Handle(extStylesheetsPath, assets.GzipHandler(extStylesheetsHandler))

	// Extension files
	for _, extConfig := range c.Options.Extensions {
		extPath := path.Join(publicURL.Path, "extensions", extConfig.Name) + "/"
		extHandler := assets.AssetExtensionHandler(extConfig.SourceDirectory, extPath, extConfig.HTML5Mode)
		mux.Handle(extPath, http.StripPrefix(extPath, extHandler))
	}

	return nil
}
