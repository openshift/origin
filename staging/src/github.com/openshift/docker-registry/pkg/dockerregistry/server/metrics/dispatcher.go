package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/docker/distribution/registry/handlers"

	gorillahandlers "github.com/gorilla/handlers"
)

// Dispatcher handles the GET requests for metrics endpoint.
func Dispatcher(ctx *handlers.Context, r *http.Request) http.Handler {
	return gorillahandlers.MethodHandler{
		"GET": prometheus.Handler(),
	}
}
