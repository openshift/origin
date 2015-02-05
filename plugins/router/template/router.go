package templaterouter

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"text/template"

	"github.com/golang/glog"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

const (
	ProtocolHTTP  = "http"
	ProtocolHTTPS = "https"
	ProtocolTLS   = "tls"
)

const (
	routeFile = "/var/lib/containers/router/routes.json"
	certDir   = "/var/lib/containers/router/certs/"
	caCertDir = "/var/lib/containers/router/cacerts/"

	caCertPostfix   = "_ca"
	destCertPostfix = "_pod"
)

// templateRouter is a backend-agnostic router implementation
// that generates configuration files via a set of templates
// and manages the backend process with a reload script.
type templateRouter struct {
	templates        map[string]*template.Template
	reloadScriptPath string
	state            map[string]ServiceUnit
	certManager      certManager
}

func newTemplateRouter(templates map[string]*template.Template, reloadScriptPath string) (*templateRouter, error) {
	router := &templateRouter{templates, reloadScriptPath, map[string]ServiceUnit{}, certManager{}}
	err := router.readState()
	return router, err
}

func (r *templateRouter) readState() error {
	dat, err := ioutil.ReadFile(routeFile)
	// XXX: rework
	if err != nil {
		r.state = make(map[string]ServiceUnit)
		return nil
	}

	return json.Unmarshal(dat, &r.state)
}

// Commit refreshes the backend and persists the router state.
func (r *templateRouter) Commit() error {
	glog.V(4).Info("Commiting router changes")

	if err := r.writeState(); err != nil {
		return err
	}

	if err := r.writeConfig(); err != nil {
		return err
	}

	if err := r.reloadRouter(); err != nil {
		return err
	}

	return nil
}

// writeState writes the state of this router to disk.
func (r *templateRouter) writeState() error {
	dat, err := json.MarshalIndent(r.state, "", "  ")
	if err != nil {
		glog.Errorf("Failed to marshal route table: %v", err)
		return err
	}
	err = ioutil.WriteFile(routeFile, dat, 0644)
	if err != nil {
		glog.Errorf("Failed to write route table: %v", err)
		return err
	}

	return nil
}

// writeConfig writes the config to disk
func (r *templateRouter) writeConfig() error {
	//write out any certificate files that don't exist
	//TODO: better way so this doesn't need to create lots of files every time state is written, probably too expensive
	for _, serviceUnit := range r.state {
		for _, cfg := range serviceUnit.ServiceAliasConfigs {
			r.certManager.writeCertificatesForConfig(&cfg)
		}
	}

	for path, template := range r.templates {
		file, err := os.Create(path)
		if err != nil {
			glog.Errorf("Error creating config file %v: %v", path, err)
			return err
		}

		err = template.Execute(file, r.state)
		if err != nil {
			glog.Errorf("Error executing template for file %v: %v", path, err)
			return err
		}

		file.Close()
	}

	return nil
}

// reloadRouter executes the router's reload script.
func (r *templateRouter) reloadRouter() error {
	cmd := exec.Command(r.reloadScriptPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		glog.Errorf("Error reloading router: %v\n Reload output: %v", err, string(out))
	}
	return err
}

// CreateServiceUnit creates a new service named with the given id.
func (r *templateRouter) CreateServiceUnit(id string) {
	service := ServiceUnit{
		Name:                id,
		ServiceAliasConfigs: make(map[string]ServiceAliasConfig),
		EndpointTable:       make(map[string]Endpoint),
	}

	r.state[id] = service
}

// FindServiceUnit finds the service with the given id.
func (r *templateRouter) FindServiceUnit(id string) (v ServiceUnit, ok bool) {
	v, ok = r.state[id]
	return
}

// DeleteFrontend deletes the service with the given id.
func (r *templateRouter) DeleteServiceUnit(id string) {
	delete(r.state, id)
}

// DeleteEndpoints deletes the endpoints for the service with the given id.
func (r *templateRouter) DeleteEndpoints(id string) {
	service, ok := r.FindServiceUnit(id)
	if !ok {
		return
	}
	service.EndpointTable = make(map[string]Endpoint)

	r.state[id] = service
}

// routeKey generates route key in form of Host-Path
func (r *templateRouter) routeKey(route *routeapi.Route) string {
	return route.Host + "-" + route.Path
}

// AddRoute adds a route for the given id
func (r *templateRouter) AddRoute(id string, route *routeapi.Route) {
	frontend, _ := r.FindServiceUnit(id)

	backendKey := r.routeKey(route)

	config := ServiceAliasConfig{
		Host: route.Host,
		Path: route.Path,
	}

	if route.TLS != nil && len(route.TLS.Termination) > 0 {
		config.TLSTermination = route.TLS.Termination

		if route.TLS.Termination != routeapi.TLSTerminationPassthrough {
			if config.Certificates == nil {
				config.Certificates = make(map[string]Certificate)
			}

			cert := Certificate{
				ID:         route.Host,
				Contents:   route.TLS.Certificate,
				PrivateKey: route.TLS.Key,
			}

			config.Certificates[cert.ID] = cert

			if len(route.TLS.CACertificate) > 0 {
				caCert := Certificate{
					ID:       route.Host + caCertPostfix,
					Contents: route.TLS.CACertificate,
				}

				config.Certificates[caCert.ID] = caCert
			}

			if len(route.TLS.DestinationCACertificate) > 0 {
				destCert := Certificate{
					ID:       route.Host + destCertPostfix,
					Contents: route.TLS.DestinationCACertificate,
				}

				config.Certificates[destCert.ID] = destCert
			}
		}
	}

	//create or replace
	frontend.ServiceAliasConfigs[backendKey] = config
	r.state[id] = frontend
}

// RemoveRoute removes the given route for the given id.
func (r *templateRouter) RemoveRoute(id string, route *routeapi.Route) {
	_, ok := r.state[id]

	if !ok {
		return
	}

	delete(r.state[id].ServiceAliasConfigs, r.routeKey(route))
}

// AddEndpoints adds new Endpoints for the given id.
func (r *templateRouter) AddEndpoints(id string, endpoints []Endpoint) {
	frontend, _ := r.FindServiceUnit(id)

	//only add if it doesn't already exist
	for _, ep := range endpoints {
		if _, ok := frontend.EndpointTable[ep.ID]; !ok {
			newEndpoint := Endpoint{ep.ID, ep.IP, ep.Port}
			frontend.EndpointTable[ep.ID] = newEndpoint
		}
	}

	r.state[id] = frontend
}

func cmpStrSlices(first []string, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for _, fi := range first {
		found := false
		for _, si := range second {
			if fi == si {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
