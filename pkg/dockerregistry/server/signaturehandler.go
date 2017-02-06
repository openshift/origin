package server

import (
	"net/http"

	"github.com/docker/distribution/context"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/auth"
	"github.com/docker/distribution/registry/handlers"
)

// RegisterSignatureHandler registers the Docker image signature extension to Docker
// registry.
func RegisterSignatureHandler(app *handlers.App) {
	extensionsRouter := app.NewRoute().PathPrefix("/extensions/v2/").Subrouter()
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
		extensionsRouter.Path("/{name:"+reference.NameRegexp.String()+"}/signatures/{digest:"+reference.DigestRegexp.String()+"}").Methods("GET"),
		SignatureDispatcher,
		handlers.NameRequired,
		getSignatureAccess,
	)
	app.RegisterRoute(
		extensionsRouter.Path("/{name:"+reference.NameRegexp.String()+"}/signatures/{digest:"+reference.DigestRegexp.String()+"}").Methods("PUT"),
		SignatureDispatcher,
		handlers.NameRequired,
		putSignatureAccess,
	)
}
