package dockerregistry

import (
	"io"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/distribution/configuration"
	ctxu "github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/handlers"
	_ "github.com/docker/distribution/registry/storage/driver/filesystem"
	_ "github.com/docker/distribution/registry/storage/driver/s3"
	"github.com/docker/distribution/version"
	gorillahandlers "github.com/gorilla/handlers"
	_ "github.com/openshift/origin/pkg/dockerregistry/middleware/repository"
	"golang.org/x/net/context"
)

// Execute runs the Docker registry.
func Execute(configFile io.Reader) {
	config, err := configuration.Parse(configFile)
	if err != nil {
		log.Fatalf("Error parsing configuration file: %s", err)
	}

	logLevel, err := log.ParseLevel(string(config.Loglevel))
	if err != nil {
		log.Errorf("Error parsing log level %q: %s", config.Loglevel, err)
		logLevel = log.InfoLevel
	}
	log.SetLevel(logLevel)

	ctx := context.Background()
	ctx = context.WithValue(ctx, "version", version.Version)
	ctx = ctxu.WithLogger(ctx, ctxu.GetLogger(ctx, "version"))

	app := handlers.NewApp(ctx, *config)
	handler := gorillahandlers.CombinedLoggingHandler(os.Stdout, app)

	if config.HTTP.TLS.Certificate == "" {
		ctxu.GetLogger(app).Infof("listening on %v", config.HTTP.Addr)
		if err := http.ListenAndServe(config.HTTP.Addr, handler); err != nil {
			ctxu.GetLogger(app).Fatalln(err)
		}
	} else {
		ctxu.GetLogger(app).Infof("listening on %v, tls", config.HTTP.Addr)
		if err := http.ListenAndServeTLS(config.HTTP.Addr, config.HTTP.TLS.Certificate, config.HTTP.TLS.Key, handler); err != nil {
			ctxu.GetLogger(app).Fatalln(err)
		}
	}
}
