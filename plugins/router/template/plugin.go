package templaterouter

import (
	"fmt"
	"strconv"
	"text/template"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	ktypes "github.com/GoogleCloudPlatform/kubernetes/pkg/types"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/golang/glog"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// TemplatePlugin implements the router.Plugin interface to provide
// a template based, backend-agnostic router.
type TemplatePlugin struct {
	Router router
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

	// AddRoute adds a route for the given id.  Returns true if a change was made
	// and the state should be stored with Commit().
	AddRoute(id string, route *routeapi.Route) bool
	// RemoveRoute removes the given route for the given id.
	RemoveRoute(id string, route *routeapi.Route)

	// Commit refreshes the backend and persists the router state.
	Commit() error
}

// NewTemplatePlugin creates a new TemplatePlugin.
func NewTemplatePlugin(templatePath, reloadScriptPath, defaultCertificate string, service ktypes.NamespacedName) (*TemplatePlugin, error) {
	masterTemplate := template.Must(template.New("config").ParseFiles(templatePath))
	templates := map[string]*template.Template{}

	for _, template := range masterTemplate.Templates() {
		if template == masterTemplate {
			continue
		}

		templates[template.Name()] = template
	}

	router, err := newTemplateRouter(templates, reloadScriptPath, defaultCertificate, peerEndpointsKey(service))
	return &TemplatePlugin{router}, err
}

// HandleEndpoints processes watch events on the Endpoints resource.
func (p *TemplatePlugin) HandleEndpoints(eventType watch.EventType, endpoints *kapi.Endpoints) error {
	key := endpointsKey(*endpoints)

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
		key := endpointsKey(*endpoints)
		commit := p.Router.AddEndpoints(key, routerEndpoints)
		if commit {
			return p.Router.Commit()
		}
	}

	return nil
}

// HandleRoute processes watch events on the Route resource.
func (p *TemplatePlugin) HandleRoute(eventType watch.EventType, route *routeapi.Route) error {
	key := routeKey(*route)
	if _, ok := p.Router.FindServiceUnit(key); !ok {
		glog.V(4).Infof("Creating new frontend for key: %v", key)
		p.Router.CreateServiceUnit(key)
	}

	switch eventType {
	case watch.Added, watch.Modified:
		glog.V(4).Infof("Modifying routes for %s", key)
		commit := p.Router.AddRoute(key, route)
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

// routeKey returns the internal router key to use for the given Route.
func routeKey(route routeapi.Route) string {
	return fmt.Sprintf("%s/%s", route.Namespace, route.ServiceName)
}

// endpointsKey returns the internal router key to use for the given Endpoints.
func endpointsKey(endpoints kapi.Endpoints) string {
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
