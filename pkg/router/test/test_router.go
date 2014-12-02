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

func (r *Router) ReadRoutes() {
	r.RoutesRead = true
}

func (r *Router) WriteRoutes() {
	r.RoutesWritten = true
}

func (r *Router) FindFrontend(name string) (router.Frontend, bool) {
	f, ok := r.FrontendsToFind[name]
	return f, ok
}

func (r *Router) DeleteBackends(name string) {
	r.DeletedBackends = append(r.DeletedBackends, name)
}

func (r *Router) CreateFrontend(name, url string) {
	r.CreatedFrontends = append(r.CreatedFrontends, name)
}

func (r *Router) DeleteFrontend(name string) {
	r.DeletedFrontends = append(r.DeletedFrontends, name)
}

func (r *Router) AddAlias(alias, frontendName string) {
	r.AddedAliases[alias] = frontendName
}

func (r *Router) RemoveAlias(alias, frontendName string) {
	r.RemovedAliases[alias] = frontendName
}

func (r *Router) AddRoute(frontend *router.Frontend, backend *router.Backend, endpoints []router.Endpoint) {
	r.AddedRoutes[frontend.Name] = endpoints
}

func (r *Router) WriteConfig() {
	r.ConfigWritten = true
}

func (r *Router) ReloadRouter() bool {
	r.RouterReloaded = true
	return true
}
