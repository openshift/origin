package dockerregistry

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/health"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/auth"
	"github.com/docker/distribution/registry/handlers"
	_ "github.com/docker/distribution/registry/storage/driver/filesystem"
	_ "github.com/docker/distribution/registry/storage/driver/s3"
	"github.com/docker/distribution/version"
	gorillahandlers "github.com/gorilla/handlers"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/dockerregistry/server"
)

// Execute runs the Docker registry.
func Execute(configFile io.Reader) {
	config, err := configuration.Parse(configFile)
	if err != nil {
		log.Fatalf("Error parsing configuration file: %s", err)
	}

	logLevel, err := log.ParseLevel(string(config.Log.Level))
	if err != nil {
		log.Errorf("Error parsing log level %q: %s", config.Log.Level, err)
		logLevel = log.InfoLevel
	}
	log.SetLevel(logLevel)

	log.Infof("version=%s", version.Version)
	ctx := context.Background()

	app := handlers.NewApp(ctx, config)

	// TODO add https scheme
	adminRouter := app.NewRoute().PathPrefix("/admin/").Subrouter()

	pruneAccessRecords := func(*http.Request) []auth.Access {
		return []auth.Access{
			{
				Resource: auth.Resource{
					Type: "admin",
				},
				Action: "prune",
			},
		}
	}

	app.RegisterRoute(
		// DELETE /admin/blobs/<digest>
		adminRouter.Path("/blobs/{digest:"+reference.DigestRegexp.String()+"}").Methods("DELETE"),
		// handler
		server.BlobDispatcher,
		// repo name not required in url
		handlers.NameNotRequired,
		// custom access records
		pruneAccessRecords,
	)

	app.RegisterRoute(
		// DELETE /admin/<repo>/manifests/<digest>
		adminRouter.Path("/{name:"+reference.NameRegexp.String()+"}/manifests/{digest:"+digest.DigestRegexp.String()+"}").Methods("DELETE"),
		// handler
		server.ManifestDispatcher,
		// repo name required in url
		handlers.NameRequired,
		// custom access records
		pruneAccessRecords,
	)

	app.RegisterRoute(
		// DELETE /admin/<repo>/layers/<digest>
		adminRouter.Path("/{name:"+reference.NameRegexp.String()+"}/layers/{digest:"+digest.DigestRegexp.String()+"}").Methods("DELETE"),
		// handler
		server.LayerDispatcher,
		// repo name required in url
		handlers.NameRequired,
		// custom access records
		pruneAccessRecords,
	)

	app.RegisterHealthChecks()
	handler := alive("/", app)
	handler = health.Handler(handler)
	handler = panicHandler(handler)
	handler = gorillahandlers.CombinedLoggingHandler(os.Stdout, handler)

	if config.HTTP.TLS.Certificate == "" {
		context.GetLogger(app).Infof("listening on %v", config.HTTP.Addr)
		if err := http.ListenAndServe(config.HTTP.Addr, handler); err != nil {
			context.GetLogger(app).Fatalln(err)
		}
	} else {
		tlsConf := crypto.SecureTLSConfig(&tls.Config{ClientAuth: tls.NoClientCert})

		if len(config.HTTP.TLS.ClientCAs) != 0 {
			pool := x509.NewCertPool()

			for _, ca := range config.HTTP.TLS.ClientCAs {
				caPem, err := ioutil.ReadFile(ca)
				if err != nil {
					context.GetLogger(app).Fatalln(err)
				}

				if ok := pool.AppendCertsFromPEM(caPem); !ok {
					context.GetLogger(app).Fatalln(fmt.Errorf("Could not add CA to pool"))
				}
			}

			for _, subj := range pool.Subjects() {
				context.GetLogger(app).Debugf("CA Subject: %s", string(subj))
			}

			tlsConf.ClientAuth = tls.RequireAndVerifyClientCert
			tlsConf.ClientCAs = pool
		}

		context.GetLogger(app).Infof("listening on %v, tls", config.HTTP.Addr)
		server := &http.Server{
			Addr:      config.HTTP.Addr,
			Handler:   handler,
			TLSConfig: tlsConf,
		}

		if err := server.ListenAndServeTLS(config.HTTP.TLS.Certificate, config.HTTP.TLS.Key); err != nil {
			context.GetLogger(app).Fatalln(err)
		}
	}
}

// alive simply wraps the handler with a route that always returns an http 200
// response when the path is matched. If the path is not matched, the request
// is passed to the provided handler. There is no guarantee of anything but
// that the server is up. Wrap with other handlers (such as health.Handler)
// for greater affect.
func alive(path string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == path {
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(http.StatusOK)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

// panicHandler add a HTTP handler to web app. The handler recover the happening
// panic. logrus.Panic transmits panic message to pre-config log hooks, which is
// defined in config.yml.
func panicHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Panic(fmt.Sprintf("%v", err))
			}
		}()
		handler.ServeHTTP(w, r)
	})
}
