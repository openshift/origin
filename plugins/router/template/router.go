package templaterouter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/util"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

const (
	ProtocolHTTP  = "http"
	ProtocolHTTPS = "https"
	ProtocolTLS   = "tls"
)

const (
	routeFile       = "routes.json"
	certDir         = "certs"
	caCertDir       = "cacerts"
	defaultCertName = "default"

	caCertPostfix   = "_ca"
	destCertPostfix = "_pod"
)

// templateRouter is a backend-agnostic router implementation
// that generates configuration files via a set of templates
// and manages the backend process with a reload script.
type templateRouter struct {
	// the directory to write router output to
	dir              string
	templates        map[string]*template.Template
	reloadScriptPath string
	state            map[string]ServiceUnit
	certManager      certificateManager
	// defaultCertificate is a concatenated certificate(s), their keys, and their CAs that should be used by the underlying
	// implementation as the default certificate if no certificate is resolved by the normal matching mechanisms.  This is
	// usually a wildcard certificate for a cloud domain such as *.mypaas.com to allow applications to create app.mypaas.com
	// as secure routes without having to provide their own certificates
	defaultCertificate string
	// if the default certificate is populated then this will be filled in so it can be passed to the templates
	defaultCertificatePath string
	// peerService provides a namespace/name to check against when receiving endpoint events in order
	// to track the peers of this router.  This may be used to populate the set of peer ip addresses
	// that a router can use for talking to other routers controlled by the same service.
	// NOTE: this should follow the format of the router.endpointsKey that is used to key endpoints
	peerEndpointsKey string
	// peerEndpoints will contain an endpoint slice of the peers
	peerEndpoints []Endpoint
	// if the router can expose statistics it should expose them with this user for auth
	statsUser string
	// if the router can expose statistics it should expose them with this password for auth
	statsPassword string
	// if the router can expose statistics it should expose them with this port
	statsPort int
}

// templateRouterCfg holds all configuration items required to initialize the template router
type templateRouterCfg struct {
	dir                string
	templates          map[string]*template.Template
	reloadScriptPath   string
	defaultCertificate string
	statsUser          string
	statsPassword      string
	statsPort          int
	peerEndpointsKey   string
}

// templateConfig is a subset of the templateRouter information that should be passed to the template for generating
// the correct configuration.
type templateData struct {
	// the directory that files will be written to, defaults to /var/lib/containers/router
	WorkingDir string
	// the routes
	State map[string]ServiceUnit
	// full path and file name to the default certificate
	DefaultCertificate string
	// peers
	PeerEndpoints []Endpoint
	//username to expose stats with (if the template supports it)
	StatsUser string
	//password to expose stats with (if the template supports it)
	StatsPassword string
	//port to expose stats with (if the template supports it)
	StatsPort int
}

func newTemplateRouter(cfg templateRouterCfg) (*templateRouter, error) {
	dir := cfg.dir

	glog.V(2).Infof("Creating a new template router, writing to %s", dir)
	if len(cfg.peerEndpointsKey) > 0 {
		glog.V(2).Infof("Router will use %s service to identify peers", cfg.peerEndpointsKey)
	}
	certManagerConfig := &certificateManagerConfig{
		certKeyFunc:     generateCertKey,
		caCertKeyFunc:   generateCACertKey,
		destCertKeyFunc: generateDestCertKey,
		certDir:         filepath.Join(dir, certDir),
		caCertDir:       filepath.Join(dir, caCertDir),
	}
	certManager, err := newSimpleCertificateManager(certManagerConfig, newSimpleCertificateWriter())
	if err != nil {
		return nil, err
	}

	router := &templateRouter{
		dir:                    dir,
		templates:              cfg.templates,
		reloadScriptPath:       cfg.reloadScriptPath,
		state:                  make(map[string]ServiceUnit),
		certManager:            certManager,
		defaultCertificate:     cfg.defaultCertificate,
		defaultCertificatePath: "",
		statsUser:              cfg.statsUser,
		statsPassword:          cfg.statsPassword,
		statsPort:              cfg.statsPort,
		peerEndpointsKey:       cfg.peerEndpointsKey,
		peerEndpoints:          []Endpoint{},
	}
	if err := router.writeDefaultCert(); err != nil {
		return nil, err
	}
	glog.V(4).Infof("Reading persisted state")
	if err := router.readState(); err != nil {
		return nil, err
	}
	glog.V(4).Infof("Committing state")
	if err := router.Commit(); err != nil {
		return nil, err
	}
	return router, nil
}

// writeDefaultCert is called a single time during init to write out the default certificate
func (r *templateRouter) writeDefaultCert() error {
	if len(r.defaultCertificate) == 0 {
		return nil
	}

	dir := filepath.Join(r.dir, certDir)
	glog.V(2).Infof("Writing default certificate to %s", dir)
	if err := r.certManager.CertificateWriter().WriteCertificate(dir, defaultCertName, []byte(r.defaultCertificate)); err != nil {
		return err
	}
	r.defaultCertificatePath = filepath.Join(dir, fmt.Sprintf("%s.pem", defaultCertName))
	return nil
}

func (r *templateRouter) readState() error {
	data, err := ioutil.ReadFile(filepath.Join(r.dir, routeFile))
	// TODO: rework
	if err != nil {
		r.state = make(map[string]ServiceUnit)
		return nil
	}

	return json.Unmarshal(data, &r.state)
}

// Commit refreshes the backend and persists the router state.
func (r *templateRouter) Commit() error {
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
	data, err := json.MarshalIndent(r.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal route table: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(r.dir, routeFile), data, 0644); err != nil {
		return fmt.Errorf("failed to write route table: %v", err)
	}
	return nil
}

// writeConfig writes the config to disk
func (r *templateRouter) writeConfig() error {
	//write out any certificate files that don't exist
	for _, serviceUnit := range r.state {
		for k, cfg := range serviceUnit.ServiceAliasConfigs {
			if err := r.writeCertificates(&cfg); err != nil {
				return fmt.Errorf("error writing certificates for %s: %v", serviceUnit.Name, err)
			}
			cfg.Status = ServiceAliasConfigStatusSaved
			serviceUnit.ServiceAliasConfigs[k] = cfg
		}
	}

	for path, template := range r.templates {
		file, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("error creating config file %s: %v", path, err)
		}

		data := templateData{
			WorkingDir:         r.dir,
			State:              r.state,
			DefaultCertificate: r.defaultCertificatePath,
			PeerEndpoints:      r.peerEndpoints,
			StatsUser:          r.statsUser,
			StatsPassword:      r.statsPassword,
			StatsPort:          r.statsPort,
		}
		if err := template.Execute(file, data); err != nil {
			file.Close()
			return fmt.Errorf("error executing template for file %s: %v", path, err)
		}
		file.Close()
	}

	return nil
}

// writeCertificates attempts to write certificates only if the cfg requires it see shouldWriteCerts
// for details
func (r *templateRouter) writeCertificates(cfg *ServiceAliasConfig) error {
	if r.shouldWriteCerts(cfg) {
		//TODO: better way so this doesn't need to create lots of files every time state is written, probably too expensive
		return r.certManager.WriteCertificatesForConfig(cfg)
	}
	return nil
}

// reloadRouter executes the router's reload script.
func (r *templateRouter) reloadRouter() error {
	cmd := exec.Command(r.reloadScriptPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error reloading router: %v\n---\n%s", err, string(out))
	}
	return nil
}

func (r *templateRouter) FilterNamespaces(namespaces util.StringSet) {
	if len(namespaces) == 0 {
		r.state = make(map[string]ServiceUnit)
	}
	for k := range r.state {
		// TODO: the id of a service unit should be defined inside this class, not passed in from the outside
		//   remove the leak of the abstraction when we refactor this code
		ns := strings.SplitN(k, "/", 2)[0]
		if namespaces.Has(ns) {
			continue
		}
		delete(r.state, k)
	}
}

// CreateServiceUnit creates a new service named with the given id.
func (r *templateRouter) CreateServiceUnit(id string) {
	service := ServiceUnit{
		Name:                id,
		ServiceAliasConfigs: make(map[string]ServiceAliasConfig),
		EndpointTable:       []Endpoint{},
	}

	r.state[id] = service
}

// FindServiceUnit finds the service with the given id.
func (r *templateRouter) FindServiceUnit(id string) (ServiceUnit, bool) {
	v, ok := r.state[id]
	return v, ok
}

// DeleteServiceUnit deletes the service with the given id.
func (r *templateRouter) DeleteServiceUnit(id string) {
	svcUnit, ok := r.FindServiceUnit(id)
	if !ok {
		return
	}

	for _, cfg := range svcUnit.ServiceAliasConfigs {
		r.cleanUpServiceAliasConfig(&cfg)
	}
	delete(r.state, id)
}

// DeleteEndpoints deletes the endpoints for the service with the given id.
func (r *templateRouter) DeleteEndpoints(id string) {
	service, ok := r.FindServiceUnit(id)
	if !ok {
		return
	}
	service.EndpointTable = []Endpoint{}

	r.state[id] = service

	// TODO: this is not safe (assuming that the subset of elements we are watching includes the peer endpoints)
	// should be a DNS lookup for endpoints of our service name.
	if id == r.peerEndpointsKey {
		r.peerEndpoints = []Endpoint{}
		glog.V(4).Infof("Peer endpoint table has been cleared")
	}
}

// routeKey generates route key in form of Namespace_Name.  This is NOT the normal key structure of ns/name because
// it is not safe to use / in names of router config files.  This allows templates to use this key without having
// to create (or provide) a separate method
func (r *templateRouter) routeKey(route *routeapi.Route) string {
	// Namespace can contain dashes, so ${namespace}-${name} is not
	// unique, use an underscore instead - ${namespace}_${name} akin
	// to the way domain keys/service records use it ala
	// _$service.$proto.$name.
	// Note here that underscore (_) is not a valid DNS character and
	// is just used for the key name and not for the record/route name.
	// This also helps the use case for the key used as a router config
	// file name.
	return fmt.Sprintf("%s_%s", route.Namespace, route.Name)
}

// AddRoute adds a route for the given id
func (r *templateRouter) AddRoute(id string, route *routeapi.Route, host string) bool {
	frontend, _ := r.FindServiceUnit(id)

	backendKey := r.routeKey(route)

	config := ServiceAliasConfig{
		Host: host,
		Path: route.Spec.Path,
	}

	tls := route.Spec.TLS
	if tls != nil && len(tls.Termination) > 0 {
		config.TLSTermination = tls.Termination

		if tls.Termination != routeapi.TLSTerminationPassthrough {
			if config.Certificates == nil {
				config.Certificates = make(map[string]Certificate)
			}

			certKey := generateCertKey(&config)
			cert := Certificate{
				ID:         backendKey,
				Contents:   tls.Certificate,
				PrivateKey: tls.Key,
			}

			config.Certificates[certKey] = cert

			if len(tls.CACertificate) > 0 {
				caCertKey := generateCACertKey(&config)
				caCert := Certificate{
					ID:       backendKey,
					Contents: tls.CACertificate,
				}

				config.Certificates[caCertKey] = caCert
			}

			if len(tls.DestinationCACertificate) > 0 {
				destCertKey := generateDestCertKey(&config)
				destCert := Certificate{
					ID:       backendKey,
					Contents: tls.DestinationCACertificate,
				}

				config.Certificates[destCertKey] = destCert
			}
		}
	}

	//create or replace
	frontend.ServiceAliasConfigs[backendKey] = config
	r.state[id] = frontend
	r.cleanUpdates(id, backendKey)
	return true
}

// cleanUpdates ensures the route is only under a single service key.  Backends are keyed
// by route namespace and name.  Frontends are keyed by service namespace name.  This accounts
// for times when someone updates the service name on a route which leaves the existing old service
// in state.
// TODO: remove this when we refactor the model to use existing objects and integrate this into
// the api somehow.
func (r *templateRouter) cleanUpdates(frontendKey string, backendKey string) {
	for k, v := range r.state {
		if k == frontendKey {
			continue
		}
		for routeKey := range v.ServiceAliasConfigs {
			if routeKey == backendKey {
				delete(v.ServiceAliasConfigs, backendKey)
			}
		}
	}
}

// RemoveRoute removes the given route for the given id.
func (r *templateRouter) RemoveRoute(id string, route *routeapi.Route) {
	serviceUnit, ok := r.state[id]
	if !ok {
		return
	}

	routeKey := r.routeKey(route)
	serviceAliasConfig, ok := serviceUnit.ServiceAliasConfigs[routeKey]
	if !ok {
		return
	}
	r.cleanUpServiceAliasConfig(&serviceAliasConfig)
	delete(r.state[id].ServiceAliasConfigs, routeKey)
}

// AddEndpoints adds new Endpoints for the given id.
func (r *templateRouter) AddEndpoints(id string, endpoints []Endpoint) bool {
	frontend, _ := r.FindServiceUnit(id)

	//only make the change if there is a difference
	if reflect.DeepEqual(frontend.EndpointTable, endpoints) {
		glog.V(4).Infof("Ignoring change for %s, endpoints are the same", id)
		return false
	}

	frontend.EndpointTable = endpoints
	r.state[id] = frontend

	if id == r.peerEndpointsKey {
		r.peerEndpoints = frontend.EndpointTable
		glog.V(4).Infof("Peer endpoints updated to: %#v", r.peerEndpoints)
	}

	return true
}

// cleanUpServiceAliasConfig performs any necessary steps to clean up a service alias config before deleting it from
// the router.  Right now the only clean up step is to remove any of the certificates on disk.
func (r *templateRouter) cleanUpServiceAliasConfig(cfg *ServiceAliasConfig) {
	err := r.certManager.DeleteCertificatesForConfig(cfg)
	if err != nil {
		glog.Errorf("Error deleting certificates for route %s, the route will still be deleted but files may remain in the container: %v", cfg.Host, err)
	}
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

// shouldWriteCerts determines if the router should ask the cert manager to write out certificates
// it will return true if a route is edge or reencrypt and it has all the required (host/key) certificates
// defined.  If the route does not have the certificates defined it will log an info message if the
// router is configured with a default certificate and assume the route is meant to be a wildcard.  Otherwise
// it will log a warning.  The route will still be written but users may receive browser errors
// for a host/cert mismatch
func (r *templateRouter) shouldWriteCerts(cfg *ServiceAliasConfig) bool {
	if cfg.Certificates == nil {
		return false
	}

	if cfg.TLSTermination == routeapi.TLSTerminationEdge || cfg.TLSTermination == routeapi.TLSTerminationReencrypt {
		if hasRequiredEdgeCerts(cfg) {
			return true
		} else {
			msg := fmt.Sprintf("a %s terminated route with host %s does not have the required certificates.  The route will still be created but no certificates will be written",
				cfg.TLSTermination, cfg.Host)
			// if a default cert is configured we'll assume it is meant to be a wildcard and only log info
			// otherwise we'll consider this a warning
			if len(r.defaultCertificate) > 0 {
				glog.V(4).Info(msg)
			} else {
				glog.Warning(msg)
			}
			return false
		}
	}
	return false
}

// hasRequiredEdgeCerts ensures that at least a host certificate and key are provided.
// a ca cert is not required because it may be something that is in the root cert chain
func hasRequiredEdgeCerts(cfg *ServiceAliasConfig) bool {
	hostCert, ok := cfg.Certificates[cfg.Host]
	if ok && len(hostCert.Contents) > 0 && len(hostCert.PrivateKey) > 0 {
		return true
	}
	return false
}

func generateCertKey(config *ServiceAliasConfig) string {
	return config.Host
}

func generateCACertKey(config *ServiceAliasConfig) string {
	return config.Host + caCertPostfix
}

func generateDestCertKey(config *ServiceAliasConfig) string {
	return config.Host + destCertPostfix
}
