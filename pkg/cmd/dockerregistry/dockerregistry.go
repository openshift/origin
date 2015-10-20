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
	"github.com/docker/distribution/registry/auth"
	"github.com/docker/distribution/registry/handlers"
	_ "github.com/docker/distribution/registry/storage/driver/filesystem"
	_ "github.com/docker/distribution/registry/storage/driver/s3"
	"github.com/docker/distribution/version"
	gorillahandlers "github.com/gorilla/handlers"
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

	app := handlers.NewApp(ctx, *config)

	// register OpenShift routes
	// TODO: change this to an anonymous Access record
	app.RegisterRoute(app.NewRoute().Path("/healthz"), server.HealthzHandler, handlers.NameNotRequired, handlers.NoCustomAccessRecords)

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
		// POST /admin/prune
		adminRouter.Path("/prune").Methods("POST"),
		// handler
		server.PruneHandler,
		// repo name not required in url
		handlers.NameNotRequired,
		// custom access records
		pruneAccessRecords,
	)

	handler := gorillahandlers.CombinedLoggingHandler(os.Stdout, app)

	if config.HTTP.TLS.Certificate == "" {
		context.GetLogger(app).Infof("listening on %v", config.HTTP.Addr)
		if err := http.ListenAndServe(config.HTTP.Addr, handler); err != nil {
			context.GetLogger(app).Fatalln(err)
		}
	} else {
		tlsConf := &tls.Config{
			ClientAuth: tls.NoClientCert,
		}

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
