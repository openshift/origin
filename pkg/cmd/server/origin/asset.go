package origin

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/elazarl/go-bindata-assetfs"
	"github.com/emicklei/go-restful"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/assets"
	"github.com/openshift/origin/pkg/assets/java"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/version"
	"k8s.io/kubernetes/pkg/util"
)

// InstallAPI adds handlers for serving static assets into the provided mux,
// then returns an array of strings indicating what endpoints were started
// (these are format strings that will expect to be sent a single string value).
func (c *AssetConfig) InstallAPI(container *restful.Container) []string {
	assetHandler, err := c.buildHandler()
	if err != nil {
		glog.Fatal(err)
	}

	publicURL, err := url.Parse(c.Options.PublicURL)
	if err != nil {
		glog.Fatal(err)
	}

	container.Handle(publicURL.Path, http.StripPrefix(publicURL.Path, assetHandler))

	return []string{fmt.Sprintf("Started Web Console %%s%s", publicURL.Path)}
}

// Run starts an http server for the static assets listening on the configured
// bind address
func (c *AssetConfig) Run() {
	assetHandler, err := c.buildHandler()
	if err != nil {
		glog.Fatal(err)
	}

	publicURL, err := url.Parse(c.Options.PublicURL)
	if err != nil {
		glog.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle(publicURL.Path, http.StripPrefix(publicURL.Path, assetHandler))
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

func (c *AssetConfig) buildHandler() (http.Handler, error) {
	assets.RegisterMimeTypes()

	masterURL, err := url.Parse(c.Options.MasterPublicURL)
	if err != nil {
		return nil, err
	}

	publicURL, err := url.Parse(c.Options.PublicURL)
	if err != nil {
		glog.Fatal(err)
	}

	config := assets.WebConsoleConfig{
		MasterAddr:        masterURL.Host,
		MasterPrefix:      OpenShiftAPIPrefix,
		KubernetesAddr:    masterURL.Host,
		KubernetesPrefix:  KubernetesAPIPrefix,
		OAuthAuthorizeURI: OpenShiftOAuthAuthorizeURL(masterURL.String()),
		OAuthRedirectBase: c.Options.PublicURL,
		OAuthClientID:     OpenShiftWebConsoleClientID,
		LogoutURI:         c.Options.LogoutURL,
	}

	assetFunc := assets.JoinAssetFuncs(assets.Asset, java.Asset)
	assetDirFunc := assets.JoinAssetDirFuncs(assets.AssetDir, java.AssetDir)

	handler := http.FileServer(&assetfs.AssetFS{assetFunc, assetDirFunc, ""})

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

	// Generated config.js can not be cached since it changes depending on startup options
	handler = assets.GeneratedConfigHandler(config, handler)

	// Gzip first so that inner handlers can react to the addition of the Vary header
	handler = assets.GzipHandler(handler)

	return handler, nil
}
