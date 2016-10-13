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

// RouteAdmissionFunc determines whether or not to admit a route.
type RouteAdmissionFunc func(*routeapi.Route) (error, bool)

// SubdomainToRouteMap contains all routes associated with a subdomain -
// fully qualified and wildcard routes.
type SubdomainToRouteMap map[string][]*routeapi.Route

// RemoveRoute removes any existing route(s) for a subdomain.
func (srm SubdomainToRouteMap) RemoveRoute(key string, route *routeapi.Route) bool {
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

func (srm SubdomainToRouteMap) InsertRoute(key string, route *routeapi.Route) {
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

	// subdomainToRoute contains all routes associated with a subdomain
	// (includes fully qualified and wildcard routes).
	subdomainToRoute SubdomainToRouteMap
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
		subdomainToRoute:  make(SubdomainToRouteMap),
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
			if subdomain := routeapi.GetSubdomainForHost(route.Spec.Host); len(subdomain) > 0 {
				p.subdomainToRoute.RemoveRoute(subdomain, route)
			}
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

// addRoute admits routes based on subdomain ownership - returns errors if the route is not admitted.
func (p *HostAdmitter) addRoute(route *routeapi.Route) error {
	subdomain := routeapi.GetSubdomainForHost(route.Spec.Host)
	if len(subdomain) == 0 {
		return nil
	}

	routeList, ok := p.subdomainToRoute[subdomain]
	if !ok {
		p.subdomainToRoute.InsertRoute(subdomain, route)
		return nil
	}

	oldest := routeList[0]
	if oldest.Namespace == route.Namespace {
		p.subdomainToRoute.InsertRoute(subdomain, route)
		return nil
	}

	// Route is in another namespace, land grab check here.
	if routeapi.RouteLessThan(oldest, route) {
		glog.V(4).Infof("Route %s cannot take subdomain %s from %s", routeNameKey(route), subdomain, routeNameKey(oldest))
		err := fmt.Errorf("a route in another namespace holds subdomain %s and is older than %s", subdomain, route.Name)
		p.recorder.RecordRouteRejection(route, "SubdomainAlreadyClaimed", err.Error())
		return err
	}

	// Namespace of this route is now the proud owner of the subdomain.
	glog.V(4).Infof("Route %s is reclaiming subdomain %s from namespace %s", routeNameKey(route), subdomain, oldest.Namespace)

	// Delete all the routes belonging to the previous "owner" (namespace).
	for idx := range routeList {
		msg := fmt.Sprintf("a route in another namespace %s owns subdomain %s", route.Namespace, subdomain)
		glog.V(4).Infof("Route %s not admitted: %s", routeNameKey(routeList[idx]), msg)
		p.recorder.RecordRouteRejection(routeList[idx], "SubdomainAlreadyClaimed", msg)
		p.plugin.HandleRoute(watch.Deleted, routeList[idx])
	}

	// And claim the subdomain.
	p.subdomainToRoute[subdomain] = []*routeapi.Route{route}
	return nil
}
