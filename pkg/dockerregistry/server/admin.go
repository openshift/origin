package server

import (
	"fmt"
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/handlers"
	"github.com/docker/distribution/registry/storage/driver/factory"
	gorillahandlers "github.com/gorilla/handlers"
)

// PruneHandler takes the request context and builds an appropriate handler for
// handling prune requests.
func PruneHandler(ctx *handlers.Context, r *http.Request) http.Handler {
	pruneHandler := &pruneHandler{
		Context: ctx,
	}

	return gorillahandlers.MethodHandler{
		"POST": http.HandlerFunc(pruneHandler.Prune),
	}
}

// pruneHandler handles http operations on registry
type pruneHandler struct {
	*handlers.Context
}

// Prune collects orphaned objects and deletes them from registry's storage
// and OpenShift's etcd store.
func (ph *pruneHandler) Prune(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	sd, err := factory.Create(ph.Config.Storage.Type(), ph.Config.Storage.Parameters())
	if err != nil {
		ph.Errors = append(ph.Errors, errcode.ErrorCodeUnknown.WithDetail(fmt.Errorf("failed to create storage driver: %v", err)))
		return
	}

	reg, err := newRegistry(ph, ph.Namespace(), nil)
	if err != nil {
		ph.Errors = append(ph.Errors, errcode.ErrorCodeUnknown.WithDetail(fmt.Errorf("failed to create registry: %v", err)))
		return
	}

	rg, err := LoadRegistryGraph(ph, reg.(*registry), sd)
	if err != nil {
		ph.Errors = append(ph.Errors, errcode.ErrorCodeUnknown.WithDetail(fmt.Errorf("failed to read registry storage: %v", err)))
		return
	}

	rg.IgnoreErrors = true
	errors := rg.PruneOrphanedObjects()
	if len(errors) > 0 {
		for _, err := range errors {
			ph.Errors = append(ph.Errors, errcode.ErrorCodeUnknown.WithDetail(fmt.Errorf("prune error: %v", err)))
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
