package server

import (
	"net/http"

	"github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/auth"
	"github.com/docker/distribution/registry/handlers"

	"github.com/openshift/origin/pkg/dockerregistry/server/api"
	"github.com/openshift/origin/pkg/dockerregistry/server/client"
)

// RegisterSignatureHandler registers the Docker image signature extension to Docker
// registry.
func RegisterSignatureHandler(app *handlers.App, isImageClient client.ImageStreamImagesNamespacer) {
	extensionsRouter := app.NewRoute().PathPrefix(api.ExtensionsPrefix).Subrouter()
	var (
		getSignatureAccess = func(r *http.Request) []auth.Access {
			return []auth.Access{
				{
					Resource: auth.Resource{
						Type: "signature",
						Name: context.GetStringValue(context.WithVars(app, r), "vars.name"),
					},
					Action: "get",
				},
			}
		}
		putSignatureAccess = func(r *http.Request) []auth.Access {
			return []auth.Access{
				{
					Resource: auth.Resource{
						Type: "signature",
						Name: context.GetStringValue(context.WithVars(app, r), "vars.name"),
					},
					Action: "put",
				},
			}
		}
	)
	app.RegisterRoute(
		extensionsRouter.Path(api.SignaturesPath).Methods("GET"),
		NewSignatureDispatcher(isImageClient),
		handlers.NameRequired,
		getSignatureAccess,
	)
	app.RegisterRoute(
		extensionsRouter.Path(api.SignaturesPath).Methods("PUT"),
		NewSignatureDispatcher(isImageClient),
		handlers.NameRequired,
		putSignatureAccess,
	)
}
