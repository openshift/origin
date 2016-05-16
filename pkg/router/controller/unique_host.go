package controller

import (
	"fmt"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
)

type HostToRouteMap map[string][]*routeapi.Route
type RouteToHostMap map[string]string

// UniqueHost implements the router.Plugin interface to provide
// a template based, backend-agnostic router.
type UniqueHost struct {
	plugin router.Plugin

	recorder RejectionRecorder

	hostToRoute HostToRouteMap
	routeToHost RouteToHostMap
}

// NewUniqueHost creates a plugin wrapper that ensures only unique routes are
// passed into the underlying plugin.
func NewUniqueHost(plugin router.Plugin, recorder RejectionRecorder) *UniqueHost {
	return &UniqueHost{
		plugin: plugin,

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
	return p.plugin.HandleEndpoints(eventType, endpoints)
}

// HandleRoute processes watch events on the Route resource.
// TODO: this function can probably be collapsed with the router itself, as a function that
//   determines which component needs to be recalculated (which template) and then does so
//   on demand.
func (p *UniqueHost) HandleRoute(eventType watch.EventType, route *routeapi.Route) error {
	routeName := routeNameKey(route)

	host := route.Spec.Host
	if len(host) == 0 {
		glog.V(4).Infof("Route %s has no host value", routeName)
		p.recorder.RecordRouteRejection(route, "NoHostValue", "no host value was defined for the route")
		return nil
	}

	// ensure hosts can only be claimed by one namespace at a time
	// TODO: this could be abstracted above this layer?
	if old, ok := p.hostToRoute[host]; ok {
		oldest := old[0]

		// multiple paths can be added from the namespace of the oldest route
		if oldest.Namespace == route.Namespace {
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
				p.hostToRoute[host] = next
			}
		}
		delete(p.routeToHost, routeName)
		return p.plugin.HandleRoute(eventType, route)
	}
	return nil
}

// HandleNamespaces updates the hostToRoute and routeToHost maps with respect to
// changes to the set of watched namespaces.
func (p *UniqueHost) HandleNamespaces(namespaces sets.String) error {
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

func (p *UniqueHost) SetLastSyncProcessed(processed bool) error {
	return p.plugin.SetLastSyncProcessed(processed)
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
