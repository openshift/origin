package router

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	routev1 "github.com/openshift/api/route/v1"
)

// Plugin is the interface the router controller dispatches watch events
// for the Routes and Endpoints resources to.
type Plugin interface {
	HandleRoute(watch.EventType, *routev1.Route) error
	HandleEndpoints(watch.EventType, *kapi.Endpoints) error
	// If sent, filter the list of accepted routes and endpoints to this set
	HandleNamespaces(namespaces sets.String) error
	HandleNode(watch.EventType, *kapi.Node) error
	Commit() error
}
