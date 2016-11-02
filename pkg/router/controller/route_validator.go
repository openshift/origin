package controller

import (
	"strings"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
)

// RouteHostFunc returns a host for a route. It may return an empty string.
type RouteHostFunc func(*routeapi.Route) string

// HostForRoute returns the host set on the route.
func HostForRoute(route *routeapi.Route) string {
	return route.Spec.Host
}

// routeSelectionFunc returns a function that returns the host for a route,
// possibly generated according to the specified template.
func routeSelectionFunc(hostnameTemplate string,
	overrideHostname bool) RouteHostFunc {
	if len(hostnameTemplate) == 0 {
		return HostForRoute
	}
	return func(route *routeapi.Route) string {
		if !overrideHostname && len(route.Spec.Host) > 0 {
			return route.Spec.Host
		}
		s, err := variable.ExpandStrict(hostnameTemplate,
			func(key string) (string, bool) {
				switch key {
				case "name":
					return route.Name, true
				case "namespace":
					return route.Namespace, true
				default:
					return "", false
				}
			})
		if err != nil {
			return ""
		}
		return strings.Trim(s, "\"'")
	}
}

// RouteValidator implements the router.Plugin interface and validates
// routes before passing them along to another router.Plugin.
type RouteValidator struct {
	plugin router.Plugin

	recorder RejectionRecorder

	hostForRoute RouteHostFunc

	// nil means different than empty
	allowedNamespaces sets.String
}

// NewRouteValidator creates a plugin wrapper that validates properties of
// the route before passing it into the underlying plugin.
func NewRouteValidator(plugin router.Plugin, hostnameTemplate string,
	overrideHostname bool, recorder RejectionRecorder) *RouteValidator {
	return &RouteValidator{
		plugin:       plugin,
		hostForRoute: routeSelectionFunc(hostnameTemplate, overrideHostname),

		recorder: recorder,
	}
}

// HandleEndpoints processes watch events on the Endpoints resource.
func (p *RouteValidator) HandleEndpoints(eventType watch.EventType,
	endpoints *kapi.Endpoints) error {
	if p.allowedNamespaces != nil && !p.allowedNamespaces.Has(endpoints.Namespace) {
		return nil
	}
	return p.plugin.HandleEndpoints(eventType, endpoints)
}

// HandleRoute processes watch events on the Route resource.
func (p *RouteValidator) HandleRoute(eventType watch.EventType,
	route *routeapi.Route) error {
	if p.allowedNamespaces != nil && !p.allowedNamespaces.Has(route.Namespace) {
		return nil
	}

	host := p.hostForRoute(route)
	if len(host) == 0 {
		glog.V(4).Infof("Route %s has no host value", routeNameKey(route))
		p.recorder.RecordRouteRejection(route, "NoHostValue", "no host value was defined for the route")
		return nil
	}
	route.Spec.Host = host

	switch eventType {
	case watch.Added, watch.Modified:
		return p.plugin.HandleRoute(eventType, route)

	case watch.Deleted:
		return p.plugin.HandleRoute(eventType, route)
	}
	return nil
}

// HandleNamespaces limits the scope of valid routes to only those that match
// the provided namespace list.
func (p *RouteValidator) HandleNamespaces(namespaces sets.String) error {
	p.allowedNamespaces = namespaces
	return p.plugin.HandleNamespaces(namespaces)
}

func (p *RouteValidator) SetLastSyncProcessed(processed bool) error {
	return p.plugin.SetLastSyncProcessed(processed)
}
