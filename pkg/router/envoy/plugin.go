package envoy

import (
	"fmt"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"
	corelisters "k8s.io/kubernetes/pkg/client/listers/core/internalversion"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	"github.com/openshift/origin/pkg/router"
)

type Plugin struct {
	lock sync.RWMutex

	routes map[namespacedKey]*routeapi.Route

	domainToRoute map[string][]*routeapi.Route

	servicesToRoute map[namespacedKey][]*routeapi.Route

	endpointsLister corelisters.EndpointsLister

	versions versions
	updates  *updates
}

type versions struct {
	route           int64
	servicesToRoute int64
	domainToRoute   int64
	endpoints       int64
}

func (v versions) combinedEndpoints() int64 { return v.route + v.endpoints }

type namespacedKey struct {
	namespace string
	name      string
}

func NewPlugin() *Plugin {
	return &Plugin{
		routes:          make(map[namespacedKey]*routeapi.Route),
		domainToRoute:   make(map[string][]*routeapi.Route),
		servicesToRoute: make(map[namespacedKey][]*routeapi.Route),
		updates:         newUpdates(),
	}
}

var _ router.Plugin = &Plugin{}

func (p *Plugin) HandleNamespaces(namespaces sets.String) error { return nil }
func (p *Plugin) HandleNode(watch.EventType, *kapi.Node) error  { return nil }
func (p *Plugin) Commit() error                                 { return nil }

func (p *Plugin) HandleRoute(t watch.EventType, route *routeapi.Route) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	initial := p.versions
	switch t {
	case watch.Added, watch.Modified:
		p.indexRoute(route, true)
		p.indexByDomain(route, true)
		p.indexByService(route, true)
	case watch.Deleted:
		p.indexByService(route, false)
		p.indexByDomain(route, false)
		p.indexRoute(route, false)
	default:
		return fmt.Errorf("unexpected watch type %v", t)
	}
	final := p.versions
	if initial.route != final.route {
		p.updates.Notify(envoyApiCluster, route.Namespace+"_"+route.Name)
	}
	if initial.domainToRoute != final.domainToRoute {
		p.updates.Notify(envoyApiRouteConfiguration, "openshift_http")
	}
	return nil
}

func (p *Plugin) HandleEndpoints(t watch.EventType, endpoints *kapi.Endpoints) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.versions.servicesToRoute++
	p.versions.endpoints++
	if routes, ok := p.servicesToRoute[namespacedKey{namespace: endpoints.Namespace, name: endpoints.Name}]; ok {
		names := make([]string, 0, len(routes))
		for _, route := range routes {
			names = append(names, route.Namespace+"_"+route.Name)
		}
		p.updates.Notify(envoyApiClusterLoadAssignment, names...)
	}
	return nil
}

func (p *Plugin) indexByDomain(route *routeapi.Route, add bool) {
	host := route.Spec.Host
	routes := p.domainToRoute[host]
	for i, existing := range routes {
		if existing.Name == route.Name && existing.Namespace == route.Namespace {
			if add {
				if existing.ResourceVersion == route.ResourceVersion {
					return
				}
				newRoutes := make([]*routeapi.Route, len(routes))
				copy(newRoutes, routes)
				newRoutes[i] = route
				p.domainToRoute[host] = newRoutes
			} else {
				if len(routes) <= 1 {
					delete(p.domainToRoute, host)
				} else {
					newRoutes := make([]*routeapi.Route, len(routes))
					copy(newRoutes, routes[:i])
					copy(newRoutes[i:], routes[i+1:])
					newRoutes = newRoutes[:len(newRoutes)-1]
					p.domainToRoute[host] = newRoutes
				}
			}
			p.versions.domainToRoute++
			return
		}
	}
	if add {
		routes = append(routes, route)
		p.domainToRoute[host] = routes
		p.versions.domainToRoute++
	}
}

func (p *Plugin) indexRoute(route *routeapi.Route, add bool) {
	key := namespacedKey{namespace: route.Namespace, name: route.Name}
	existing, ok := p.routes[key]
	if add {
		if ok && existing.ResourceVersion == route.ResourceVersion {
			return
		}
		p.routes[key] = route
		p.versions.route++
	} else {
		if !ok {
			return
		}
		delete(p.routes, key)
		p.versions.route++
	}
}

func (p *Plugin) indexByService(route *routeapi.Route, add bool) {
	changed := false
	services := serviceNamesForRoute(route)
Service:
	for _, service := range services {
		routes := p.servicesToRoute[service]
		for i, existing := range routes {
			if existing.Name == route.Name && existing.Namespace == route.Namespace {
				newRoutes := make([]*routeapi.Route, len(routes))
				if add {
					if existing.ResourceVersion == route.ResourceVersion {
						continue Service
					}
					copy(newRoutes, routes)
					newRoutes[i] = route
					p.servicesToRoute[service] = newRoutes
				} else {
					if len(routes) <= 1 {
						delete(p.servicesToRoute, service)
					} else {
						copy(newRoutes, routes[:i])
						copy(newRoutes[i:], routes[i+1:])
						newRoutes = newRoutes[:len(newRoutes)-1]
						p.servicesToRoute[service] = newRoutes
					}
				}
				changed = true
				continue Service
			}
		}
		if add {
			routes = append(routes, route)
			p.servicesToRoute[service] = routes
			changed = true
		}
	}
	if changed {
		p.versions.servicesToRoute++
	}
}

func serviceNamesForRoute(route *routeapi.Route) []namespacedKey {
	var names []namespacedKey
	ref := &route.Spec.To
	if ref.Kind == "Service" {
		names = append(names, namespacedKey{namespace: route.Namespace, name: ref.Name})
	}
	for i := range route.Spec.AlternateBackends {
		ref := &route.Spec.AlternateBackends[i]
		if ref.Kind == "Service" {
			names = append(names, namespacedKey{namespace: route.Namespace, name: ref.Name})
		}
	}
	return names
}

func (p *Plugin) SetListers(endpoints corelisters.EndpointsLister) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.endpointsLister = endpoints
}

type routeEndpoints struct {
	Route     *routeapi.Route
	Endpoints []*kapi.Endpoints
}

func (p *Plugin) listEndpoints(allowMissing bool, names ...string) ([]routeEndpoints, int64) {
	p.lock.RLock()
	var routes []routeEndpoints
	if len(names) > 0 {
		routes = make([]routeEndpoints, 0, len(names))
		for _, name := range names {
			parts := strings.SplitN(name, "_", 2)
			route, ok := p.routes[namespacedKey{namespace: parts[0], name: parts[1]}]
			if !ok {
				if allowMissing {
					routes = append(routes, routeEndpoints{Route: &routeapi.Route{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: parts[0],
							Name:      parts[1],
						},
					}})
				}
				continue
			}
			routes = append(routes, routeEndpoints{Route: route})
		}
	} else {
		routes = make([]routeEndpoints, 0, len(p.routes))
		for _, route := range p.routes {
			routes = append(routes, routeEndpoints{Route: route})
		}
	}
	version := p.versions.combinedEndpoints()
	p.lock.RUnlock()

	for i := range routes {
		routeBackend := &routes[i]
		route := routeBackend.Route
		endpoints := make([]*kapi.Endpoints, 0, 1+len(route.Spec.AlternateBackends))
		lister := p.endpointsLister.Endpoints(route.Namespace)
		if ept, err := lister.Get(route.Spec.To.Name); err == nil {
			endpoints = append(endpoints, ept)
		}
		for _, alt := range route.Spec.AlternateBackends {
			if ept, err := lister.Get(alt.Name); err == nil {
				endpoints = append(endpoints, ept)
			}
		}
		routeBackend.Endpoints = endpoints
	}
	return routes, version
}

func (p *Plugin) getVersions() versions {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.versions
}

func (p *Plugin) listRoutes() ([]*routeapi.Route, int64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	routes := make([]*routeapi.Route, 0, len(p.servicesToRoute))
	for _, route := range p.routes {
		routes = append(routes, route)
	}
	return routes, p.versions.route
}
