package templaterouter

import (
	"fmt"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	ktypes "k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// TemplatePlugin implements the router.Plugin interface to provide
// a template based, backend-agnostic router.
type TemplatePlugin struct {
	Router routerInterface
}

func newDefaultTemplatePlugin(router routerInterface) *TemplatePlugin {
	return &TemplatePlugin{
		Router: router,
	}
}

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

// routerInterface controls the interaction of the plugin with the underlying router implementation
type routerInterface interface {
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
	// Reduce the list of routes to only these namespaces
	FilterNamespaces(namespaces util.StringSet)
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
// TODO: this function can probably be collapsed with the router itself, as a function that
//   determines which component needs to be recalculated (which template) and then does so
//   on demand.
func (p *TemplatePlugin) HandleRoute(eventType watch.EventType, route *routeapi.Route) error {
	key := routeKey(route)

	host := route.Spec.Host

	switch eventType {
	case watch.Added, watch.Modified:
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
		p.Router.RemoveRoute(key, route)
		return p.Router.Commit()
	}
	return nil
}

// HandleAllowedNamespaces limits the scope of valid routes to only those that match
// the provided namespace list.
func (p *TemplatePlugin) HandleNamespaces(namespaces util.StringSet) error {
	p.Router.FilterNamespaces(namespaces)
	return p.Router.Commit()
}

// routeKey returns the internal router key to use for the given Route.
func routeKey(route *routeapi.Route) string {
	return fmt.Sprintf("%s/%s", route.Namespace, route.Spec.To.Name)
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
