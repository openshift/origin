package router

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// Plugin is the interface the router controller dispatches watch events
// for the Routes and Endpoints resources to.
type Plugin interface {
	HandleRoute(watch.EventType, *routeapi.Route) error
	HandleEndpoints(watch.EventType, *kapi.Endpoints) error
}
