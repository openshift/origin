package origin

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/elazarl/go-bindata-assetfs"
	"github.com/emicklei/go-restful"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/assets"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/version"
)

// InstallSupport registers endpoints for an OAuth2 server into the provided mux,
// then returns an array of strings indicating what endpoints were started
// (these are format strings that will expect to be sent a single string value).
func (c *AssetConfig) InstallAPI(container *restful.Container) []string {
	assetHandler, err := c.handler()
	if err != nil {
		glog.Fatal(err)
	}

	publicURL, err := url.Parse(c.Options.PublicURL)
	if err != nil {
		glog.Fatal(err)
	}

	mux := container.ServeMux
	mux.Handle(publicURL.Path, http.StripPrefix(publicURL.Path, assetHandler))

	return []string{fmt.Sprintf("Started OpenShift UI %%s%s", publicURL.Path)}
}

func (c *AssetConfig) Run() {
	assetHandler, err := c.handler()
	if err != nil {
		glog.Fatal(err)
	}

	publicURL, err := url.Parse(c.Options.PublicURL)
	if err != nil {
		glog.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle(publicURL.Path, http.StripPrefix(publicURL.Path, assetHandler))
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, publicURL.Path, http.StatusFound)
	})

	server := &http.Server{
		Addr:           c.Options.ServingInfo.BindAddress,
		Handler:        mux,
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	isTLS := configapi.UseTLS(c.Options.ServingInfo)

	go util.Forever(func() {
		if isTLS {
			server.TLSConfig = &tls.Config{
				// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
				MinVersion: tls.VersionTLS10,
			}
			glog.Infof("OpenShift UI listening at https://%s", c.Options.ServingInfo.BindAddress)
			glog.Fatal(server.ListenAndServeTLS(c.Options.ServingInfo.ServerCert.CertFile, c.Options.ServingInfo.ServerCert.KeyFile))
		} else {
			glog.Infof("OpenShift UI listening at https://%s", c.Options.ServingInfo.BindAddress)
			glog.Fatal(server.ListenAndServe())
		}
	}, 0)

	// Attempt to verify the server came up for 20 seconds (100 tries * 100ms, 100ms timeout per try)
	cmdutil.WaitForSuccessfulDial(isTLS, "tcp", c.Options.ServingInfo.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)

	glog.Infof("OpenShift UI available at %s", c.Options.PublicURL)
}

func (c *AssetConfig) handler() (http.Handler, error) {
	assets.RegisterMimeTypes()

	masterURL, err := url.Parse(c.Options.MasterPublicURL)
	if err != nil {
		return nil, err
	}

	k8sURL, err := url.Parse(c.Options.KubernetesPublicURL)
	if err != nil {
		return nil, err
	}

	config := assets.WebConsoleConfig{
		MasterAddr:        masterURL.Host,
		MasterPrefix:      "/osapi", // TODO: make configurable
		KubernetesAddr:    k8sURL.Host,
		KubernetesPrefix:  "/api", // TODO: make configurable
		OAuthAuthorizeURI: OpenShiftOAuthAuthorizeURL(masterURL.String()),
		OAuthRedirectBase: c.Options.PublicURL,
		OAuthClientID:     OpenShiftWebConsoleClientID,
		LogoutURI:         c.Options.LogoutURI,
	}

	mux := http.NewServeMux()
	mux.Handle("/",
		// Gzip first so that inner handlers can react to the addition of the Vary header
		assets.GzipHandler(
			// Generated config.js can not be cached since it changes depending on startup options
			assets.GeneratedConfigHandler(
				config,
				// Cache control should happen after all Vary headers are added, but before
				// any asset related routing (HTML5ModeHandler and FileServer)
				assets.CacheControlHandler(
					version.Get().GitCommit,
					assets.HTML5ModeHandler(
						http.FileServer(
							&assetfs.AssetFS{
								assets.Asset,
								assets.AssetDir,
								"",
							},
						),
					),
				),
			),
		),
	)

	return mux, nil
}
