package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/handlers"
)

// DeleteLayersRequest is a mapping from layers to the image repositories that
// reference them. Below is a sample request:
//
// {
//   "layer1": ["repo1", "repo2"],
//   "layer2": ["repo1", "repo3"],
//   ...
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
func DeleteLayersHandler(adminService distribution.AdminService) func(ctx *handlers.Context, r *http.Request) http.Handler {
	return func(ctx *handlers.Context, r *http.Request) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer req.Body.Close()
			log.Infof("deleteLayerFunc invoked")

			decoder := json.NewDecoder(req.Body)
			deletions := DeleteLayersRequest{}
			if err := decoder.Decode(&deletions); err != nil {
				//TODO
				log.Errorf("Error unmarshaling body: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

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

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(&response); err != nil {
				w.Write([]byte(fmt.Sprintf("Error marshaling response: %v", err)))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
		})
	}
}

type DeleteManifestsRequest map[string][]string

func (r DeleteManifestsRequest) AddManifest(revision string) {
	if _, ok := r[revision]; !ok {
		r[revision] = []string{}
	}
}

func (r DeleteManifestsRequest) AddStream(revision, stream string) {
	r[revision] = append(r[revision], stream)
}

type DeleteManifestsResponse struct {
	Result string
	Errors map[string][]string
}

func DeleteManifestsHandler(adminService distribution.AdminService) func(ctx *handlers.Context, r *http.Request) http.Handler {
	return func(ctx *handlers.Context, r *http.Request) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer req.Body.Close()

			decoder := json.NewDecoder(req.Body)
			deletions := DeleteManifestsRequest{}
			if err := decoder.Decode(&deletions); err != nil {
				//TODO
				log.Errorf("Error unmarshaling body: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			errs := map[string][]error{}
			for revision, repos := range deletions {
				log.Infof("Deleting manifest revision=%q, repos=%v", revision, repos)
				dgst, err := digest.ParseDigest(revision)
				if err != nil {
					errs[revision] = []error{fmt.Errorf("Error parsing revision %q: %v", revision, err)}
					continue
				}
				errs[revision] = adminService.DeleteManifest(dgst, repos)
			}

			log.Infof("errs=%v", errs)

			var result string
			switch len(errs) {
			case 0:
				result = "success"
			default:
				result = "failure"
			}

			response := DeleteManifestsResponse{
				Result: result,
				Errors: map[string][]string{},
			}

			for revision, revisionErrors := range errs {
				response.Errors[revision] = []string{}
				for _, err := range revisionErrors {
					response.Errors[revision] = append(response.Errors[revision], err.Error())
				}
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(&response); err != nil {
				w.Write([]byte(fmt.Sprintf("Error marshaling response: %v", err)))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
		})
	}
}

func DeleteRepositoryHandler(adminService distribution.AdminService) func(ctx *handlers.Context, r *http.Request) http.Handler {
	return func(ctx *handlers.Context, r *http.Request) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer req.Body.Close()

			if err := adminService.DeleteRepository(ctx.Repository.Name()); err != nil {
				w.Write([]byte(fmt.Sprintf("Error deleting repository %q: %v", ctx.Repository.Name(), err)))
			}

			w.WriteHeader(http.StatusNoContent)
		})
	}
}
