package test

import (
	"github.com/openshift/origin/pkg/router"
)

// Router provides an implementation of router.Router suitable for unit testing.
type Router struct {
	FrontendsToFind map[string]router.Frontend

	DeletedBackends  []string
	CreatedFrontends []string
	DeletedFrontends []string
	AddedAliases     map[string]string
	RemovedAliases   map[string]string
	AddedRoutes      map[string][]router.Endpoint
	RoutesRead       bool
	RoutesWritten    bool
	ConfigWritten    bool
	RouterReloaded   bool
}

// NewRouter creates a new Router.
func NewRouter(registeredFrontends map[string]router.Frontend) *Router {
	return &Router{
		FrontendsToFind:  registeredFrontends,
		DeletedBackends:  []string{},
		CreatedFrontends: []string{},
		DeletedFrontends: []string{},
		AddedAliases:     map[string]string{},
		RemovedAliases:   map[string]string{},
		AddedRoutes:      map[string][]router.Endpoint{},
	}
}

func (r *Router) ReadRoutes() error {
	r.RoutesRead = true
	return nil
}

func (r *Router) WriteRoutes() error {
	r.RoutesWritten = true
	return nil
}

func (r *Router) FindFrontend(name string) (router.Frontend, bool) {
	f, ok := r.FrontendsToFind[name]
	return f, ok
}

func (r *Router) DeleteBackends(name string) {
	r.DeletedBackends = append(r.DeletedBackends, name)
}

func (r *Router) CreateFrontend(name, url string) (*router.Frontend, error) {
	frontend := router.Frontend{
		Name:          name,
		Backends:      make(map[string]router.Backend),
		EndpointTable: make(map[string]router.Endpoint),
		HostAliases:   make([]string, 0),
	}
	r.CreatedFrontends = append(r.CreatedFrontends, name)
	return &frontend, nil
}

func (r *Router) DeleteFrontend(name string) error {
	r.DeletedFrontends = append(r.DeletedFrontends, name)
	return nil
}

func (r *Router) AddAlias(alias, frontendName string) error {
	r.AddedAliases[alias] = frontendName
	return nil
}

func (r *Router) RemoveAlias(alias, frontendName string) error {
	r.RemovedAliases[alias] = frontendName
	return nil
}

func (r *Router) AddRoute(frontend *router.Frontend, backend *router.Backend, endpoints []router.Endpoint) error {
	r.AddedRoutes[frontend.Name] = endpoints
	return nil
}

func (r *Router) WriteConfig() {
	r.ConfigWritten = true
}

func (r *Router) ReloadRouter() bool {
	r.RouterReloaded = true
	return true
}
