package controller

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	"github.com/openshift/origin/pkg/route/apis/route/validation"
	"github.com/openshift/origin/pkg/router"
)

// RouteHostFunc returns a host for a route. It may return an empty string.
type RouteHostFunc func(*routeapi.Route) string

// HostForRoute returns the host set on the route.
func HostForRoute(route *routeapi.Route) string {
	return route.Spec.Host
}

type HostToRouteMap map[string][]*routeapi.Route
type RouteToHostMap map[string]string

// UniqueHost implements the router.Plugin interface to provide
// a template based, backend-agnostic router.
type UniqueHost struct {
	plugin       router.Plugin
	hostForRoute RouteHostFunc

	recorder RejectionRecorder

	hostToRoute HostToRouteMap
	routeToHost RouteToHostMap
	// nil means different than empty
	allowedNamespaces sets.String

	disableOwnershipCheck bool
}

// NewUniqueHost creates a plugin wrapper that ensures only unique routes are passed into
// the underlying plugin. Recorder is an interface for indicating why a route was
// rejected.
func NewUniqueHost(plugin router.Plugin, fn RouteHostFunc, disableOwnershipCheck bool, recorder RejectionRecorder) *UniqueHost {
	return &UniqueHost{
		plugin:       plugin,
		hostForRoute: fn,

		disableOwnershipCheck: disableOwnershipCheck,

		recorder: recorder,

		hostToRoute: make(HostToRouteMap),
		routeToHost: make(RouteToHostMap),
	}
}

// RoutesForHost is a helper that allows routes to be retrieved.
func (p *UniqueHost) RoutesForHost(host string) ([]*routeapi.Route, bool) {
	routes, ok := p.hostToRoute[host]
	return routes, ok
}

// HostLen returns the number of hosts currently tracked by this plugin.
func (p *UniqueHost) HostLen() int {
	return len(p.hostToRoute)
}

// HandleEndpoints processes watch events on the Endpoints resource.
func (p *UniqueHost) HandleEndpoints(eventType watch.EventType, endpoints *kapi.Endpoints) error {
	if p.allowedNamespaces != nil && !p.allowedNamespaces.Has(endpoints.Namespace) {
		return nil
	}
	return p.plugin.HandleEndpoints(eventType, endpoints)
}

// HandleNode processes watch events on the Node resource and calls the router
func (p *UniqueHost) HandleNode(eventType watch.EventType, node *kapi.Node) error {
	return p.plugin.HandleNode(eventType, node)
}

// HandleRoute processes watch events on the Route resource.
// TODO: this function can probably be collapsed with the router itself, as a function that
//   determines which component needs to be recalculated (which template) and then does so
//   on demand.
func (p *UniqueHost) HandleRoute(eventType watch.EventType, route *routeapi.Route) error {
	if p.allowedNamespaces != nil && !p.allowedNamespaces.Has(route.Namespace) {
		return nil
	}

	routeName := routeNameKey(route)

	host := p.hostForRoute(route)
	if len(host) == 0 {
		glog.V(4).Infof("Route %s has no host value", routeName)
		p.recorder.RecordRouteRejection(route, "NoHostValue", "no host value was defined for the route")
		return nil
	}
	route.Spec.Host = host

	// Run time check to defend against older routes. Validate that the
	// route host name conforms to DNS requirements.
	if errs := validation.ValidateHostName(route); len(errs) > 0 {
		glog.V(4).Infof("Route %s - invalid host name %s", routeName, host)
		errMessages := make([]string, len(errs))
		for i := 0; i < len(errs); i++ {
			errMessages[i] = errs[i].Error()
		}

		err := fmt.Errorf("host name validation errors: %s", strings.Join(errMessages, ", "))
		p.recorder.RecordRouteRejection(route, "InvalidHost", err.Error())
		return err
	}

	// ensure hosts can only be claimed by one namespace at a time
	// TODO: this could be abstracted above this layer?
	if old, ok := p.hostToRoute[host]; ok {
		oldest := old[0]

		// multiple paths can be added from the namespace of the oldest route
		// unless the ownership checks are disabled.
		if p.disableOwnershipCheck || oldest.Namespace == route.Namespace {
			added := false
			for i := range old {
				if old[i].Spec.Path == route.Spec.Path {
					if routeapi.RouteLessThan(old[i], route) {
						glog.V(4).Infof("Route %s cannot take %s from %s", routeName, host, routeNameKey(oldest))
						err := fmt.Errorf("route %s already exposes %s and is older", oldest.Name, host)
						p.recorder.RecordRouteRejection(route, "HostAlreadyClaimed", err.Error())
						return err
					}
					added = true
					if old[i].Namespace == route.Namespace && old[i].Name == route.Name {
						old[i] = route
						break
					}
					glog.V(4).Infof("route %s will replace path %s from %s because it is older", routeName, route.Spec.Path, old[i].Name)
					p.recorder.RecordRouteRejection(old[i], "HostAlreadyClaimed", fmt.Sprintf("replaced by older route %s", route.Name))
					p.plugin.HandleRoute(watch.Deleted, old[i])
					old[i] = route
				}
			}
			if !added {
				// Clean out any old form of this route
				next := []*routeapi.Route{}
				for i := range old {
					if routeNameKey(old[i]) != routeNameKey(route) {
						next = append(next, old[i])
					}
				}
				old = next

				// We need to reset the oldest in case we removed it, but if it was the only
				// item, we'll just use ourselves since we'll become the oldest, and for
				// the append below, it doesn't matter
				if len(next) > 0 {
					oldest = old[0]
				} else {
					oldest = route
				}

				if routeapi.RouteLessThan(route, oldest) {
					p.hostToRoute[host] = append([]*routeapi.Route{route}, old...)
				} else {
					p.hostToRoute[host] = append(old, route)
				}
			}
		} else {
			if routeapi.RouteLessThan(oldest, route) {
				glog.V(4).Infof("Route %s cannot take %s from %s", routeName, host, routeNameKey(oldest))
				err := fmt.Errorf("a route in another namespace holds %s and is older than %s", host, route.Name)
				p.recorder.RecordRouteRejection(route, "HostAlreadyClaimed", err.Error())
				return err
			}

			glog.V(4).Infof("Route %s is reclaiming %s from namespace %s", routeName, host, oldest.Namespace)
			for i := range old {
				p.recorder.RecordRouteRejection(old[i], "HostAlreadyClaimed", fmt.Sprintf("namespace %s owns hostname %s", oldest.Namespace, host))
				p.plugin.HandleRoute(watch.Deleted, old[i])
			}
			p.hostToRoute[host] = []*routeapi.Route{route}
		}
	} else {
		glog.V(4).Infof("Route %s claims %s", routeName, host)
		p.hostToRoute[host] = []*routeapi.Route{route}
	}

	switch eventType {
	case watch.Added, watch.Modified:
		if old, ok := p.routeToHost[routeName]; ok {
			if old != host {
				glog.V(4).Infof("Route %s changed from serving host %s to host %s", routeName, old, host)
				delete(p.hostToRoute, old)
			}
		}
		p.routeToHost[routeName] = host
		return p.plugin.HandleRoute(eventType, route)

	case watch.Deleted:
		glog.V(4).Infof("Deleting routes for %s", routeName)
		if old, ok := p.hostToRoute[host]; ok {
			switch len(old) {
			case 1, 0:
				delete(p.hostToRoute, host)
			default:
				next := []*routeapi.Route{}
				for i := range old {
					if old[i].Name != route.Name {
						next = append(next, old[i])
					}
				}

				if len(next) > 0 {
					p.hostToRoute[host] = next
				} else {
					delete(p.hostToRoute, host)
				}
			}
		}
		delete(p.routeToHost, routeName)
		return p.plugin.HandleRoute(eventType, route)
	}
	return nil
}

// HandleNamespaces limits the scope of valid routes to only those that match
// the provided namespace list.
func (p *UniqueHost) HandleNamespaces(namespaces sets.String) error {
	p.allowedNamespaces = namespaces
	changed := false
	for k, v := range p.hostToRoute {
		if namespaces.Has(v[0].Namespace) {
			continue
		}
		delete(p.hostToRoute, k)
		for i := range v {
			delete(p.routeToHost, routeNameKey(v[i]))
		}
		changed = true
	}
	if !changed && len(namespaces) > 0 {
		return nil
	}
	return p.plugin.HandleNamespaces(namespaces)
}

func (p *UniqueHost) Commit() error {
	return p.plugin.Commit()
}

// routeKeys returns the internal router key to use for the given Route.
func routeKeys(route *routeapi.Route) []string {
	keys := make([]string, 1+len(route.Spec.AlternateBackends))
	keys[0] = fmt.Sprintf("%s/%s", route.Namespace, route.Spec.To.Name)
	for i, svc := range route.Spec.AlternateBackends {
		keys[i] = fmt.Sprintf("%s/%s", route.Namespace, svc.Name)
	}
	return keys
}

// routeNameKey returns a unique name for a given route
func routeNameKey(route *routeapi.Route) string {
	return fmt.Sprintf("%s/%s", route.Namespace, route.Name)
}
