package router

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// Plugin is the interface the router controller dispatches watch events
// for the Routes and Endpoints resources to.
type Plugin interface {
	HandleRoute(watch.EventType, *routeapi.Route) error
	HandleEndpoints(watch.EventType, *kapi.Endpoints) error
	// If sent, filter the list of accepted routes and endpoints to this set
	HandleNamespaces(namespaces sets.String) error
	HandleNode(watch.EventType, *kapi.Node) error
	SetLastSyncProcessed(processed bool) error
}
