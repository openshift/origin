package server

import (
	"net/http"

	"github.com/docker/distribution/health"
	"github.com/docker/distribution/registry/handlers"
)

func HealthzHandler(ctx *handlers.Context, r *http.Request) http.Handler {
	return http.HandlerFunc(health.StatusHandler)
}
