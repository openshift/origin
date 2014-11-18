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

func (r *Router) ReadRoutes() (*router.Routes, error) {
	r.RoutesRead = true
	return nil, nil
}

func (r *Router) WriteRoutes() (*router.Routes, error) {
	r.RoutesWritten = true
	return nil, nil
}

func (r *Router) FindFrontend(name string) (router.Frontend, bool) {
	f, ok := r.FrontendsToFind[name]
	return f, ok
}

func (r *Router) DeleteBackends(name string) {
	r.DeletedBackends = append(r.DeletedBackends, name)
}

func (r *Router) CreateFrontend(name, url string) (*router.Routes, error) {
	r.CreatedFrontends = append(r.CreatedFrontends, name)
	return nil, nil
}

func (r *Router) DeleteFrontend(name string) (*router.Routes, error) {
	r.DeletedFrontends = append(r.DeletedFrontends, name)
	return nil, nil
}

func (r *Router) AddAlias(alias, frontendName string) (*router.Routes, error) {
	r.AddedAliases[alias] = frontendName
	return nil, nil
}

func (r *Router) RemoveAlias(alias, frontendName string) (*router.Routes, error) {
	r.RemovedAliases[alias] = frontendName
	return nil, nil
}

func (r *Router) AddRoute(frontendName, fePath, bePath string, protocols []string, endpoints []router.Endpoint) (*router.Routes, error) {
	r.AddedRoutes[frontendName] = endpoints
	return nil, nil
}

func (r *Router) WriteConfig() {
	r.ConfigWritten = true
}

func (r *Router) ReloadRouter() bool {
	r.RouterReloaded = true
	return true
}
