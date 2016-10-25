package controller

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
)

// RouteAdmissionFunc determines whether or not to admit a route.
type RouteAdmissionFunc func(*routeapi.Route) (error, bool)

// RouteMap contains all routes associated with a key
type RouteMap map[string][]*routeapi.Route

// RemoveRoute removes any existing routes that match the given route's namespace and name for a key
func (srm RouteMap) RemoveRoute(key string, route *routeapi.Route) bool {
	k := 0
	removed := false

	m := srm[key]
	for i, v := range m {
		if m[i].Namespace == route.Namespace && m[i].Name == route.Name {
			removed = true
		} else {
			m[k] = v
			k++
		}
	}

	// set the slice length to the final size.
	m = m[:k]

	if len(m) > 0 {
		srm[key] = m
	} else {
		delete(srm, key)
	}

	return removed
}

func (srm RouteMap) InsertRoute(key string, route *routeapi.Route) {
	// To replace any existing route[s], first we remove all old entries.
	srm.RemoveRoute(key, route)

	m := srm[key]
	for idx := range m {
		if routeapi.RouteLessThan(route, m[idx]) {
			m = append(m, &routeapi.Route{})
			// From: https://github.com/golang/go/wiki/SliceTricks
			copy(m[idx+1:], m[idx:])
			m[idx] = route
			srm[key] = m

			// Ensure we return from here as we change the iterator.
			return
		}
	}

	// Newest route or empty slice, add to the end.
	srm[key] = append(m, route)
}

// HostAdmitter implements the router.Plugin interface to add admission
// control checks for routes in template based, backend-agnostic routers.
type HostAdmitter struct {
	// plugin is the next plugin in the chain.
	plugin router.Plugin

	// admitter is a route admission function used to determine whether
	// or not to admit routes.
	admitter RouteAdmissionFunc

	// recorder is an interface for indicating route rejections.
	recorder RejectionRecorder

	// restrictOwnership adds admission checks to restrict ownership
	// (of subdomains) to a single owner/namespace.
	restrictOwnership bool

	claimedHosts     RouteMap
	claimedWildcards RouteMap
	blockedWildcards RouteMap
}

// NewHostAdmitter creates a plugin wrapper that checks whether or not to
// admit routes and relay them to the next plugin in the chain.
// Recorder is an interface for indicating why a route was rejected.
func NewHostAdmitter(plugin router.Plugin, fn RouteAdmissionFunc, restrict bool, recorder RejectionRecorder) *HostAdmitter {
	return &HostAdmitter{
		plugin:   plugin,
		admitter: fn,
		recorder: recorder,

		restrictOwnership: restrict,
		claimedHosts:      RouteMap{},
		claimedWildcards:  RouteMap{},
		blockedWildcards:  RouteMap{},
	}
}

// HandleNode processes watch events on the Node resource.
func (p *HostAdmitter) HandleNode(eventType watch.EventType, node *kapi.Node) error {
	return p.plugin.HandleNode(eventType, node)
}

// HandleEndpoints processes watch events on the Endpoints resource.
func (p *HostAdmitter) HandleEndpoints(eventType watch.EventType, endpoints *kapi.Endpoints) error {
	return p.plugin.HandleEndpoints(eventType, endpoints)
}

// HandleRoute processes watch events on the Route resource.
func (p *HostAdmitter) HandleRoute(eventType watch.EventType, route *routeapi.Route) error {
	err, allow := p.admitter(route)
	if !allow {
		msg := "blocked by admitter"
		if err != nil {
			msg = err.Error()
		}

		glog.Errorf("Route %s not admitted: %s", routeNameKey(route), msg)
		p.recorder.RecordRouteRejection(route, "RouteNotAdmitted", msg)
		return err
	}

	if err != nil {
		msg := err.Error()
		glog.Warningf("Route %s admitted with warnings: %s", routeNameKey(route), msg)
		p.recorder.RecordRouteRejection(route, "RouteAdmissionWarning", msg)
	}

	if p.restrictOwnership && len(route.Spec.Host) > 0 {
		switch eventType {
		case watch.Added, watch.Modified:
			if err := p.addRoute(route); err != nil {
				glog.Errorf("Route %s not admitted: %s", routeNameKey(route), err.Error())
				p.recorder.RecordRouteRejection(route, "SubdomainAlreadyClaimed", err.Error())
				return err
			}

		case watch.Deleted:
			p.claimedHosts.RemoveRoute(route.Spec.Host, route)
			wildcardKey := getDomain(route.Spec.Host)
			p.claimedWildcards.RemoveRoute(wildcardKey, route)
			p.blockedWildcards.RemoveRoute(wildcardKey, route)
		}
	}

	return p.plugin.HandleRoute(eventType, route)
}

// HandleAllowedNamespaces limits the scope of valid routes to only those that match
// the provided namespace list.
func (p *HostAdmitter) HandleNamespaces(namespaces sets.String) error {
	return p.plugin.HandleNamespaces(namespaces)
}

func (p *HostAdmitter) SetLastSyncProcessed(processed bool) error {
	return p.plugin.SetLastSyncProcessed(processed)
}

func getDomain(hostname string) string {
	index := strings.IndexRune(hostname, '.')
	if index == -1 {
		return ""
	}
	return hostname[index+1:]
}

// addRoute admits routes based on subdomain ownership - returns errors if the route is not admitted.
func (p *HostAdmitter) addRoute(route *routeapi.Route) error {
	// Find displaced routes (or error if an existing route displaces us)
	displacedRoutes, err := p.displacedRoutes(route)
	if err != nil {
		err := fmt.Errorf("a route in another namespace holds host %s (%v)", route.Spec.Host, err)
		p.recorder.RecordRouteRejection(route, "HostAlreadyClaimed", err.Error())
		return err
	}

	// Remove displaced routes
	for _, displacedRoute := range displacedRoutes {
		wildcardKey := getDomain(displacedRoute.Spec.Host)
		p.claimedHosts.RemoveRoute(displacedRoute.Spec.Host, displacedRoute)
		p.blockedWildcards.RemoveRoute(wildcardKey, displacedRoute)
		p.claimedWildcards.RemoveRoute(wildcardKey, displacedRoute)

		msg := fmt.Sprintf("a route in another namespace holds host %s", displacedRoute.Spec.Host)
		p.recorder.RecordRouteRejection(displacedRoute, "SubdomainAlreadyClaimed", msg)
		p.plugin.HandleRoute(watch.Deleted, displacedRoute)
	}

	// Add the new route
	wildcardKey := getDomain(route.Spec.Host)

	switch route.Spec.WildcardPolicy {
	case routeapi.WildcardPolicyNone:
		// claim the host, block wildcards that would conflict with this host
		p.claimedHosts.InsertRoute(route.Spec.Host, route)
		p.blockedWildcards.InsertRoute(wildcardKey, route)
		// ensure the route doesn't exist as a claimed wildcard (in case it previously was)
		p.claimedWildcards.RemoveRoute(wildcardKey, route)

	case routeapi.WildcardPolicySubdomain:
		// claim the wildcard
		p.claimedWildcards.InsertRoute(wildcardKey, route)
		// ensure the route doesn't exist as a claimed host or blocked wildcard
		p.claimedHosts.RemoveRoute(route.Spec.Host, route)
		p.blockedWildcards.RemoveRoute(wildcardKey, route)
	}

	return nil
}

func (p *HostAdmitter) displacedRoutes(newRoute *routeapi.Route) ([]*routeapi.Route, error) {
	displaced := []*routeapi.Route{}

	// See if any existing routes block our host, or if we displace their host
	for i, route := range p.claimedHosts[newRoute.Spec.Host] {
		if route.Namespace == newRoute.Namespace {
			// Never displace a route in our namespace
			continue
		}
		if routeapi.RouteLessThan(route, newRoute) {
			return nil, fmt.Errorf("route %s/%s has host %s", route.Namespace, route.Name, route.Spec.Host)
		}
		displaced = append(displaced, p.claimedHosts[newRoute.Spec.Host][i])
	}

	wildcardKey := getDomain(newRoute.Spec.Host)

	// See if any existing wildcard routes block our domain, or if we displace them
	for i, route := range p.claimedWildcards[wildcardKey] {
		if route.Namespace == newRoute.Namespace {
			// Never displace a route in our namespace
			continue
		}
		if routeapi.RouteLessThan(route, newRoute) {
			return nil, fmt.Errorf("wildcard route %s/%s has host *.%s, blocking %s", route.Namespace, route.Name, wildcardKey, newRoute.Spec.Host)
		}
		displaced = append(displaced, p.claimedWildcards[wildcardKey][i])
	}

	// If this is a wildcard route, see if any specific hosts block our wildcardSpec, or if we displace them
	if newRoute.Spec.WildcardPolicy == routeapi.WildcardPolicySubdomain {
		for i, route := range p.blockedWildcards[wildcardKey] {
			if route.Namespace == newRoute.Namespace {
				// Never displace a route in our namespace
				continue
			}
			if routeapi.RouteLessThan(route, newRoute) {
				return nil, fmt.Errorf("route %s/%s has host %s, blocking *.%s", route.Namespace, route.Name, route.Spec.Host, wildcardKey)
			}
			displaced = append(displaced, p.blockedWildcards[wildcardKey][i])
		}
	}

	return displaced, nil
}
