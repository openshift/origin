package router

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

type ProbeType string

const (
	LivenessProbe  ProbeType = "LivenessProbe"
	ReadinessProbe ProbeType = "ReadinessProbe"
)

// Plugin is the interface the router controller dispatches watch events
// for the Routes and Endpoints resources to.
type Plugin interface {
	HandleRoute(watch.EventType, *routeapi.Route) error
	HandleEndpoints(watch.EventType, *kapi.Endpoints) error
	// If sent, filter the list of accepted routes and endpoints to this set
	HandleNamespaces(namespaces sets.String) error
	HandleNode(watch.EventType, *kapi.Node) error
	HandleProbe(probe ProbeType, timeout time.Duration) (int, []byte)
	Commit() error
}
