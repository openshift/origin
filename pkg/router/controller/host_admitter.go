package controller

import (
	"fmt"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	"github.com/openshift/origin/pkg/router"
)

// RouteAdmissionFunc determines whether or not to admit a route.
type RouteAdmissionFunc func(*routeapi.Route) error

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

	// allowWildcardRoutes enables wildcard route support.
	allowWildcardRoutes bool

	// disableNamespaceCheck disables admission checks to restrict
	// ownership (of subdomains) to a single owner/namespace.
	disableNamespaceCheck bool

	claimedHosts     RouteMap
	claimedWildcards RouteMap
	blockedWildcards RouteMap
}

// NewHostAdmitter creates a plugin wrapper that checks whether or not to
// admit routes and relay them to the next plugin in the chain.
// Recorder is an interface for indicating why a route was rejected.
func NewHostAdmitter(plugin router.Plugin, fn RouteAdmissionFunc, allowWildcards, disableNamespaceCheck bool, recorder RejectionRecorder) *HostAdmitter {
	return &HostAdmitter{
		plugin:   plugin,
		admitter: fn,
		recorder: recorder,

		allowWildcardRoutes:   allowWildcards,
		disableNamespaceCheck: disableNamespaceCheck,

		claimedHosts:     RouteMap{},
		claimedWildcards: RouteMap{},
		blockedWildcards: RouteMap{},
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
	if err := p.admitter(route); err != nil {
		glog.V(4).Infof("Route %s not admitted: %s", routeNameKey(route), err.Error())
		p.recorder.RecordRouteRejection(route, "RouteNotAdmitted", err.Error())
		return err
	}

	if p.allowWildcardRoutes && len(route.Spec.Host) > 0 {
		switch eventType {
		case watch.Added, watch.Modified:
			if err := p.addRoute(route); err != nil {
				glog.Errorf("Route %s not admitted: %s", routeNameKey(route), err.Error())
				return err
			}

		case watch.Deleted:
			p.claimedHosts.RemoveRoute(route.Spec.Host, route)
			wildcardKey := routeapi.GetDomainForHost(route.Spec.Host)
			p.claimedWildcards.RemoveRoute(wildcardKey, route)
			p.blockedWildcards.RemoveRoute(wildcardKey, route)
		}
	}

	return p.plugin.HandleRoute(eventType, route)
}

// HandleNamespaces limits the scope of valid routes to only those that match
// the provided namespace list.
func (p *HostAdmitter) HandleNamespaces(namespaces sets.String) error {
	return p.plugin.HandleNamespaces(namespaces)
}

func (p *HostAdmitter) Commit() error {
	return p.plugin.Commit()
}

// addRoute admits routes based on subdomain ownership - returns errors if the route is not admitted.
func (p *HostAdmitter) addRoute(route *routeapi.Route) error {
	// Find displaced routes (or error if an existing route displaces us)
	displacedRoutes, err, ownerNamespace := p.displacedRoutes(route)
	if err != nil {
		msg := fmt.Sprintf("a route in another namespace holds host %s", route.Spec.Host)
		if ownerNamespace == route.Namespace {
			// Use the full error details if we got bumped by a
			// route in our namespace.
			msg = err.Error()
		}
		p.recorder.RecordRouteRejection(route, "HostAlreadyClaimed", msg)
		return err
	}

	// Remove displaced routes
	for _, displacedRoute := range displacedRoutes {
		wildcardKey := routeapi.GetDomainForHost(displacedRoute.Spec.Host)
		p.claimedHosts.RemoveRoute(displacedRoute.Spec.Host, displacedRoute)
		p.blockedWildcards.RemoveRoute(wildcardKey, displacedRoute)
		p.claimedWildcards.RemoveRoute(wildcardKey, displacedRoute)

		msg := ""
		if route.Namespace == displacedRoute.Namespace {
			if route.Spec.WildcardPolicy == routeapi.WildcardPolicySubdomain {
				msg = fmt.Sprintf("wildcard route %s/%s has host *.%s blocking %s", route.Namespace, route.Name, wildcardKey, displacedRoute.Spec.Host)
			} else {
				msg = fmt.Sprintf("route %s/%s has host %s, blocking %s", route.Namespace, route.Name, route.Spec.Host, displacedRoute.Spec.Host)
			}
		} else {
			msg = fmt.Sprintf("a route in another namespace holds host %s", displacedRoute.Spec.Host)
		}

		p.recorder.RecordRouteRejection(displacedRoute, "HostAlreadyClaimed", msg)
		p.plugin.HandleRoute(watch.Deleted, displacedRoute)
	}

	if len(route.Spec.WildcardPolicy) == 0 {
		route.Spec.WildcardPolicy = routeapi.WildcardPolicyNone
	}

	// Add the new route
	wildcardKey := routeapi.GetDomainForHost(route.Spec.Host)

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
	default:
		p.claimedHosts.RemoveRoute(route.Spec.Host, route)
		p.claimedWildcards.RemoveRoute(wildcardKey, route)
		p.blockedWildcards.RemoveRoute(wildcardKey, route)
		err := fmt.Errorf("unsupported wildcard policy %s", route.Spec.WildcardPolicy)
		p.recorder.RecordRouteRejection(route, "RouteNotAdmitted", err.Error())
		return err
	}

	return nil
}

func (p *HostAdmitter) displacedRoutes(newRoute *routeapi.Route) ([]*routeapi.Route, error, string) {
	displaced := []*routeapi.Route{}

	// See if any existing routes block our host, or if we displace their host
	for i, route := range p.claimedHosts[newRoute.Spec.Host] {
		if p.disableNamespaceCheck || route.Namespace == newRoute.Namespace {
			if !p.disableNamespaceCheck && route.Name == newRoute.Name {
				continue
			}

			// Check for wildcard routes. Never displace a
			// non-wildcard route in our namespace if we are a
			// wildcard route.
			// E.g. *.acme.test can co-exist with a
			//      route for www2.acme.test
			if newRoute.Spec.WildcardPolicy == routeapi.WildcardPolicySubdomain {
				continue
			}

			// New route is _NOT_ a wildcard - we need to check
			// if the paths are same and if it is older before
			// we can displace another route in our namespace.
			// The path check below allows non-wildcard routes
			// with different paths to coexist with the other
			// non-wildcard routes in this namespace.
			// E.g. www.acme.org/p1 can coexist with other
			//      non-wildcard routes www.acme.org/p1/p2 or
			//      www.acme.org/p2 or www.acme.org/p2/p3
			//      but ...
			//      not with www.acme.org/p1
			if route.Spec.Path != newRoute.Spec.Path {
				continue
			}
		}
		if routeapi.RouteLessThan(route, newRoute) {
			return nil, fmt.Errorf("route %s/%s has host %s", route.Namespace, route.Name, route.Spec.Host), route.Namespace
		}
		displaced = append(displaced, p.claimedHosts[newRoute.Spec.Host][i])
	}

	wildcardKey := routeapi.GetDomainForHost(newRoute.Spec.Host)

	// See if any existing wildcard routes block our domain, or if we displace them
	for i, route := range p.claimedWildcards[wildcardKey] {
		if p.disableNamespaceCheck || route.Namespace == newRoute.Namespace {
			if !p.disableNamespaceCheck && route.Name == newRoute.Name {
				continue
			}

			// Check for non-wildcard route. Never displace a
			// wildcard route in our namespace if we are not a
			// wildcard route.
			// E.g. www1.foo.test can co-exist with a
			//      wildcard route for *.foo.test
			if newRoute.Spec.WildcardPolicy != routeapi.WildcardPolicySubdomain {
				continue
			}

			// New route is a wildcard - we need to check if the
			// paths are same and if it is older before we can
			// displace another route in our namespace.
			// The path check below allows wildcard routes with
			// different paths to coexist with the other wildcard
			// routes in this namespace.
			// E.g. *.bar.org/p1 can coexist with other wildcard
			//      wildcard routes *.bar.org/p1/p2 or
			//      *.bar.org/p2 or *.bar.org/p2/p3
			//      but ...
			//      not with *.bar.org/p1
			if route.Spec.Path != newRoute.Spec.Path {
				continue
			}
		}
		if routeapi.RouteLessThan(route, newRoute) {
			return nil, fmt.Errorf("wildcard route %s/%s has host *.%s, blocking %s", route.Namespace, route.Name, wildcardKey, newRoute.Spec.Host), route.Namespace
		}
		displaced = append(displaced, p.claimedWildcards[wildcardKey][i])
	}

	// If this is a wildcard route, see if any specific hosts block our wildcardSpec, or if we displace them
	if newRoute.Spec.WildcardPolicy == routeapi.WildcardPolicySubdomain {
		for i, route := range p.blockedWildcards[wildcardKey] {
			if p.disableNamespaceCheck || route.Namespace == newRoute.Namespace {
				// Never displace a route in our namespace
				continue
			}
			if routeapi.RouteLessThan(route, newRoute) {
				return nil, fmt.Errorf("route %s/%s has host %s, blocking *.%s", route.Namespace, route.Name, route.Spec.Host, wildcardKey), route.Namespace
			}
			displaced = append(displaced, p.blockedWildcards[wildcardKey][i])
		}
	}

	return displaced, nil, newRoute.Namespace
}
