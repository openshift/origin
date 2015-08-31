package templaterouter

import (
	"fmt"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	ktypes "k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// TemplatePlugin implements the router.Plugin interface to provide
// a template based, backend-agnostic router.
type TemplatePlugin struct {
	Router       router
	HostForRoute func(*routeapi.Route) string
	hostToRoute  HostToRouteMap
	routeToHost  RouteToHostMap
}

func newDefaultTemplatePlugin(router router) *TemplatePlugin {
	return &TemplatePlugin{
		Router:       router,
		HostForRoute: defaultHostForRoute,
		hostToRoute:  make(HostToRouteMap),
		routeToHost:  make(RouteToHostMap),
	}
}

type HostToRouteMap map[string]*routeapi.Route
type RouteToHostMap map[string]string

type TemplatePluginConfig struct {
	WorkingDir         string
	TemplatePath       string
	ReloadScriptPath   string
	DefaultCertificate string
	StatsPort          int
	StatsUsername      string
	StatsPassword      string
	PeerService        *ktypes.NamespacedName
}

// router controls the interaction of the plugin with the underlying router implementation
type router interface {
	// Mutative operations in this interface do not return errors.
	// The only error state for these methods is when an unknown
	// frontend key is used; all call sites make certain the frontend
	// is created.

	// CreateServiceUnit creates a new service named with the given id.
	CreateServiceUnit(id string)
	// FindServiceUnit finds the service with the given id.
	FindServiceUnit(id string) (v ServiceUnit, ok bool)

	// AddEndpoints adds new Endpoints for the given id.Returns true if a change was made
	// and the state should be stored with Commit().
	AddEndpoints(id string, endpoints []Endpoint) bool
	// DeleteEndpoints deletes the endpoints for the frontend with the given id.
	DeleteEndpoints(id string)

	// AddRoute adds a route for the given id and the calculated host.  Returns true if a
	// change was made and the state should be stored with Commit().
	AddRoute(id string, route *routeapi.Route, host string) bool
	// RemoveRoute removes the given route for the given id.
	RemoveRoute(id string, route *routeapi.Route)

	// Commit refreshes the backend and persists the router state.
	Commit() error
}

// NewTemplatePlugin creates a new TemplatePlugin.
func NewTemplatePlugin(cfg TemplatePluginConfig) (*TemplatePlugin, error) {
	templateBaseName := filepath.Base(cfg.TemplatePath)
	masterTemplate, err := template.New("config").ParseFiles(cfg.TemplatePath)
	if err != nil {
		return nil, err
	}
	templates := map[string]*template.Template{}

	for _, template := range masterTemplate.Templates() {
		if template.Name() == templateBaseName {
			continue
		}

		templates[template.Name()] = template
	}

	peerKey := ""
	if cfg.PeerService != nil {
		peerKey = peerEndpointsKey(*cfg.PeerService)
	}

	templateRouterCfg := templateRouterCfg{
		dir:                cfg.WorkingDir,
		templates:          templates,
		reloadScriptPath:   cfg.ReloadScriptPath,
		defaultCertificate: cfg.DefaultCertificate,
		statsUser:          cfg.StatsUsername,
		statsPassword:      cfg.StatsPassword,
		statsPort:          cfg.StatsPort,
		peerEndpointsKey:   peerKey,
	}
	router, err := newTemplateRouter(templateRouterCfg)
	return newDefaultTemplatePlugin(router), err
}

// HandleEndpoints processes watch events on the Endpoints resource.
func (p *TemplatePlugin) HandleEndpoints(eventType watch.EventType, endpoints *kapi.Endpoints) error {
	key := endpointsKey(endpoints)

	glog.V(4).Infof("Processing %d Endpoints for Name: %v (%v)", len(endpoints.Subsets), endpoints.Name, eventType)

	for i, s := range endpoints.Subsets {
		glog.V(4).Infof("  Subset %d : %#v", i, s)
	}

	if _, ok := p.Router.FindServiceUnit(key); !ok {
		p.Router.CreateServiceUnit(key)
	}

	switch eventType {
	case watch.Added, watch.Modified:
		glog.V(4).Infof("Modifying endpoints for %s", key)
		routerEndpoints := createRouterEndpoints(endpoints)
		key := endpointsKey(endpoints)
		commit := p.Router.AddEndpoints(key, routerEndpoints)
		if commit {
			return p.Router.Commit()
		}
	}

	return nil
}

// HandleRoute processes watch events on the Route resource.
func (p *TemplatePlugin) HandleRoute(eventType watch.EventType, route *routeapi.Route) error {
	key := routeKey(route)
	routeName := routeNameKey(route)

	host := p.HostForRoute(route)
	if len(host) == 0 {
		return nil
	}

	// ensure hosts can only be claimed by one route at a time
	// TODO: this could be abstracted above this layer?
	if old, ok := p.hostToRoute[host]; ok {
		if old.CreationTimestamp.Before(route.CreationTimestamp) {
			glog.V(4).Infof("Route %s cannot take %s from %s", routeName, host, routeNameKey(old))
			return fmt.Errorf("route %s holds %s and is older than %s", routeNameKey(old), host, key)
		}
		glog.V(4).Infof("Route %s is reclaiming %s from %s", routeName, host, routeNameKey(old))
		p.Router.RemoveRoute(routeKey(old), old)
		p.hostToRoute[host] = route
	} else {
		glog.V(4).Infof("Route %s claims %s", key, host)
		p.hostToRoute[host] = route
	}

	switch eventType {
	case watch.Added, watch.Modified:
		// TODO: this could be abstracted above this layer?
		if old, ok := p.routeToHost[routeName]; ok {
			if old != host {
				glog.V(4).Infof("Route %s changed from serving host %s to host %s", key, old, host)
				delete(p.hostToRoute, old)
			}
		}
		p.routeToHost[routeName] = host

		if _, ok := p.Router.FindServiceUnit(key); !ok {
			glog.V(4).Infof("Creating new frontend for key: %v", key)
			p.Router.CreateServiceUnit(key)
		}

		glog.V(4).Infof("Modifying routes for %s", key)
		commit := p.Router.AddRoute(key, route, host)
		if commit {
			return p.Router.Commit()
		}
	case watch.Deleted:
		glog.V(4).Infof("Deleting routes for %s", key)
		delete(p.hostToRoute, host)
		delete(p.routeToHost, routeName)
		p.Router.RemoveRoute(key, route)
		return p.Router.Commit()
	}
	return nil
}

// defaultHostForRoute return the host based on the string value on a route.
func defaultHostForRoute(route *routeapi.Route) string {
	return route.Host
}

// routeKey returns the internal router key to use for the given Route.
func routeKey(route *routeapi.Route) string {
	return fmt.Sprintf("%s/%s", route.Namespace, route.ServiceName)
}

// routeNameKey returns a unique name for a given route
func routeNameKey(route *routeapi.Route) string {
	return fmt.Sprintf("%s/%s", route.Namespace, route.Name)
}

// endpointsKey returns the internal router key to use for the given Endpoints.
func endpointsKey(endpoints *kapi.Endpoints) string {
	return fmt.Sprintf("%s/%s", endpoints.Namespace, endpoints.Name)
}

// peerServiceKey may be used by the underlying router when handling endpoints to identify
// endpoints that belong to its peers.  THIS MUST FOLLOW THE KEY STRATEGY OF endpointsKey.  It
// receives a NamespacedName that is created from the service that is added by the oadm command
func peerEndpointsKey(namespacedName ktypes.NamespacedName) string {
	return fmt.Sprintf("%s/%s", namespacedName.Namespace, namespacedName.Name)
}

// createRouterEndpoints creates openshift router endpoints based on k8s endpoints
func createRouterEndpoints(endpoints *kapi.Endpoints) []Endpoint {
	out := make([]Endpoint, 0, len(endpoints.Subsets)*4)

	// TODO: review me for sanity
	for _, s := range endpoints.Subsets {
		for _, a := range s.Addresses {
			for _, p := range s.Ports {
				ep := Endpoint{
					ID:   fmt.Sprintf("%s:%d", a.IP, p.Port),
					IP:   a.IP,
					Port: strconv.Itoa(p.Port),
				}
				if a.TargetRef != nil {
					ep.TargetName = a.TargetRef.Name
				} else {
					ep.TargetName = ep.IP
				}
				out = append(out, ep)
			}
		}
	}

	return out
}
