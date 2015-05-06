package dockerregistry

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/distribution/configuration"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/health"
	"github.com/docker/distribution/registry/handlers"
	_ "github.com/docker/distribution/registry/storage/driver/filesystem"
	_ "github.com/docker/distribution/registry/storage/driver/s3"
	"github.com/docker/distribution/version"
	gorillahandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/openshift/origin/pkg/dockerregistry/server"
)

func newOpenShiftHandler(app *handlers.App) http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/healthz", health.StatusHandler)
	// TODO add https scheme
	router.HandleFunc("/admin/layers", deleteLayerFunc(app)).Methods("DELETE")
	//router.HandleFunc("/admin/manifests", deleteManifestFunc(app)).Methods("DELETE")
	// delegate to the registry if it's not 1 of the OpenShift routes
	router.NotFoundHandler = app

	return router
}

// DeleteLayersRequest is a mapping from layers to the image repositories that
// reference them. Below is a sample request:
//
// {
//   "layer1": ["repo1", "repo2"],
// 	 "layer2": ["repo1", "repo3"],
// 	 ...
// }
type DeleteLayersRequest map[string][]string

// AddLayer adds a layer to the request if it doesn't already exist.
func (r DeleteLayersRequest) AddLayer(layer string) {
	if _, ok := r[layer]; !ok {
		r[layer] = []string{}
	}
}

// AddStream adds an image stream reference to the layer.
func (r DeleteLayersRequest) AddStream(layer, stream string) {
	r[layer] = append(r[layer], stream)
}

type DeleteLayersResponse struct {
	Result string
	Errors map[string][]string
}

// deleteLayerFunc returns an http.HandlerFunc that is able to fully delete a
// layer from storage.
func deleteLayerFunc(app *handlers.App) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		log.Infof("deleteLayerFunc invoked")

		//TODO verify auth

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			//TODO
			log.Errorf("Error reading body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		deletions := DeleteLayersRequest{}
		err = json.Unmarshal(body, &deletions)
		if err != nil {
			//TODO
			log.Errorf("Error unmarshaling body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		adminService := app.Registry().AdminService()
		errs := map[string][]error{}
		for layer, repos := range deletions {
			log.Infof("Deleting layer=%q, repos=%v", layer, repos)
			errs[layer] = adminService.DeleteLayer(layer, repos)
		}

		log.Infof("errs=%v", errs)

		var result string
		switch len(errs) {
		case 0:
			result = "success"
		default:
			result = "failure"
		}

		response := DeleteLayersResponse{
			Result: result,
			Errors: map[string][]string{},
		}

		for layer, layerErrors := range errs {
			response.Errors[layer] = []string{}
			for _, err := range layerErrors {
				response.Errors[layer] = append(response.Errors[layer], err.Error())
			}
		}

		buf, err := json.Marshal(&response)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("Error marshaling response: %v", err)))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Write(buf)
		w.WriteHeader(http.StatusOK)
	}
}

/*
type DeleteManifestsRequest map[string][]string

func (r *DeleteManifestsRequest) AddManifest(revision string) {
	if _, ok := r[revision]; !ok {
		r[revision] = []string{}
	}
}

func (r *DeleteManifestsRequest) AddStream(revision, stream string) {
	r[revision] = append(r[revision], stream)
}

func deleteManifestsFunc(app *handlers.App) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()

		//TODO verify auth

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			//TODO
			log.Errorf("Error reading body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		deletions := DeleteManifestsRequest{}
		err = json.Unmarshal(body, &deletions)
		if err != nil {
			//TODO
			log.Errorf("Error unmarshaling body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		adminService := app.Registry().AdminService()
		errs := []error{}
		for revision, repos := range deletions {
			log.Infof("Deleting manifest revision=%q, repos=%v", revision, repos)
			manifestErrs := adminService.DeleteManifest(revision, repos)
			errs = append(errs, manifestErrs...)
		}

		log.Infof("errs=%v", errs)

		//TODO write response
		w.WriteHeader(http.StatusOK)
	}
}
*/

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
	handler := newOpenShiftHandler(app)
	handler = gorillahandlers.CombinedLoggingHandler(os.Stdout, handler)

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
