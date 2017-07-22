package templaterouter

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"

	"k8s.io/apimachinery/pkg/util/sets"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	"github.com/openshift/origin/pkg/router/controller"
	"github.com/openshift/origin/pkg/util/ratelimiter"
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
	reloadInterval   time.Duration
	reloadCallbacks  []func()
	state            map[string]ServiceAliasConfig
	serviceUnits     map[string]ServiceUnit
	certManager      certificateManager
	// defaultCertificate is a concatenated certificate(s), their keys, and their CAs that should be used by the underlying
	// implementation as the default certificate if no certificate is resolved by the normal matching mechanisms.  This is
	// usually a wildcard certificate for a cloud domain such as *.mypaas.com to allow applications to create app.mypaas.com
	// as secure routes without having to provide their own certificates
	defaultCertificate string
	// if the default certificate is populated then this will be filled in so it can be passed to the templates
	defaultCertificatePath string
	// if the default certificate is in a secret this will be filled in so it can be passed to the templates
	defaultCertificateDir string
	// defaultDestinationCAPath is a path to a CA bundle that should be used by the underlying implementation as the default
	// destination CA if no certificate is resolved by the normal matching mechanisms. This is usually the service serving
	// certificate CA (/var/run/secrets/kubernetes.io/serviceaccount/serving_ca.crt) that the infrastructure uses to
	// generate certificates for services by name.
	defaultDestinationCAPath string
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
	// if the router should allow wildcard routes.
	allowWildcardRoutes bool
	// rateLimitedCommitFunction is a rate limited commit (persist state + refresh the backend)
	// function that coalesces and controls how often the router is reloaded.
	rateLimitedCommitFunction *ratelimiter.RateLimitedFunction
	// rateLimitedCommitStopChannel is the stop/terminate channel.
	rateLimitedCommitStopChannel chan struct{}
	// lock is a mutex used to prevent concurrent router reloads.
	lock sync.Mutex
	// If true, haproxy should only bind ports when it has route and endpoint state
	bindPortsAfterSync bool
	// whether the router state has been read from the api at least once
	synced bool
	// whether a state change has occurred
	stateChanged bool
	// metricReload tracks reloads
	metricReload prometheus.Summary
	// metricWriteConfig tracks writing config
	metricWriteConfig prometheus.Summary
}

// templateRouterCfg holds all configuration items required to initialize the template router
type templateRouterCfg struct {
	dir                      string
	templates                map[string]*template.Template
	reloadScriptPath         string
	reloadInterval           time.Duration
	reloadCallbacks          []func()
	defaultCertificate       string
	defaultCertificatePath   string
	defaultCertificateDir    string
	defaultDestinationCAPath string
	statsUser                string
	statsPassword            string
	statsPort                int
	allowWildcardRoutes      bool
	peerEndpointsKey         string
	includeUDP               bool
	bindPortsAfterSync       bool
}

// templateConfig is a subset of the templateRouter information that should be passed to the template for generating
// the correct configuration.
type templateData struct {
	// the directory that files will be written to, defaults to /var/lib/containers/router
	WorkingDir string
	// the routes
	State map[string](ServiceAliasConfig)
	// the service lookup
	ServiceUnits map[string]ServiceUnit
	// full path and file name to the default certificate
	DefaultCertificate string
	// full path and file name to the default destination certificate
	DefaultDestinationCA string
	// peers
	PeerEndpoints []Endpoint
	//username to expose stats with (if the template supports it)
	StatsUser string
	//password to expose stats with (if the template supports it)
	StatsPassword string
	//port to expose stats with (if the template supports it)
	StatsPort int
	// whether the router should bind the default ports
	BindPorts bool
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

	metricsReload := prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace: "template_router",
		Name:      "reload_seconds",
		Help:      "Measures the time spent reloading the router in seconds.",
	})
	prometheus.MustRegister(metricsReload)
	metricWriteConfig := prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace: "template_router",
		Name:      "write_config_seconds",
		Help:      "Measures the time spent writing out the router configuration to disk in seconds.",
	})
	prometheus.MustRegister(metricWriteConfig)

	router := &templateRouter{
		dir:                      dir,
		templates:                cfg.templates,
		reloadScriptPath:         cfg.reloadScriptPath,
		reloadInterval:           cfg.reloadInterval,
		reloadCallbacks:          cfg.reloadCallbacks,
		state:                    make(map[string]ServiceAliasConfig),
		serviceUnits:             make(map[string]ServiceUnit),
		certManager:              certManager,
		defaultCertificate:       cfg.defaultCertificate,
		defaultCertificatePath:   cfg.defaultCertificatePath,
		defaultCertificateDir:    cfg.defaultCertificateDir,
		defaultDestinationCAPath: cfg.defaultDestinationCAPath,
		statsUser:                cfg.statsUser,
		statsPassword:            cfg.statsPassword,
		statsPort:                cfg.statsPort,
		allowWildcardRoutes:      cfg.allowWildcardRoutes,
		peerEndpointsKey:         cfg.peerEndpointsKey,
		peerEndpoints:            []Endpoint{},
		bindPortsAfterSync:       cfg.bindPortsAfterSync,

		metricReload:      metricsReload,
		metricWriteConfig: metricWriteConfig,

		rateLimitedCommitFunction:    nil,
		rateLimitedCommitStopChannel: make(chan struct{}),
	}

	numSeconds := int(cfg.reloadInterval.Seconds())
	router.EnableRateLimiter(numSeconds, router.commitAndReload)

	if err := router.writeDefaultCert(); err != nil {
		return nil, err
	}
	glog.V(4).Infof("Reading persisted state")
	if err := router.readState(); err != nil {
		return nil, err
	}
	glog.V(4).Infof("Committing state")
	// Bypass the rate limiter to ensure the first sync will be
	// committed without delay.
	router.commitAndReload()
	return router, nil
}

func isInteger(s string) bool {
	_, err := strconv.Atoi(s)
	return (err == nil)
}

func matchValues(s string, allowedValues ...string) bool {
	for _, value := range allowedValues {
		if value == s {
			return true
		}
	}
	return false
}

func matchPattern(pattern, s string) bool {
	glog.V(5).Infof("matchPattern called with %s and %s", pattern, s)
	status, err := regexp.MatchString(`\A(?:`+pattern+`)\z`, s)
	if err == nil {
		glog.V(5).Infof("matchPattern returning status: %v", status)
		return status
	}
	glog.Errorf("Error with regex pattern in call to matchPattern: %v", err)
	return false
}

// genSubdomainWildcardRegexp is now legacy and around for backward
// compatibility and allows old templates to continue running.
// Generate a regular expression to match wildcard hosts (and paths if any)
// for a [sub]domain.
func genSubdomainWildcardRegexp(hostname, path string, exactPath bool) string {
	subdomain := routeapi.GetDomainForHost(hostname)
	if len(subdomain) == 0 {
		glog.Warningf("Generating subdomain wildcard regexp - invalid host name %s", hostname)
		return fmt.Sprintf("%s%s", hostname, path)
	}

	expr := regexp.QuoteMeta(fmt.Sprintf(".%s%s", subdomain, path))
	if exactPath {
		return fmt.Sprintf(`^[^\.]*%s$`, expr)
	}

	return fmt.Sprintf(`^[^\.]*%s(|/.*)$`, expr)
}

// Generate a regular expression to match route hosts (and paths if any).
func generateRouteRegexp(hostname, path string, wildcard bool) string {
	hostRE := regexp.QuoteMeta(hostname)
	if wildcard {
		subdomain := routeapi.GetDomainForHost(hostname)
		if len(subdomain) == 0 {
			glog.Warningf("Generating subdomain wildcard regexp - invalid host name %s", hostname)
		} else {
			subdomainRE := regexp.QuoteMeta(fmt.Sprintf(".%s", subdomain))
			hostRE = fmt.Sprintf(`[^\.]*%s`, subdomainRE)
		}
	}

	portRE := "(:[0-9]+)?"

	// build the correct subpath regex, depending on whether path ends with a segment separator
	var pathRE, subpathRE string
	switch {
	case strings.TrimRight(path, "/") == "":
		// Special-case paths consisting solely of "/" to match a root request to "" as well
		pathRE = ""
		subpathRE = "(/.*)?"
	case strings.HasSuffix(path, "/"):
		pathRE = regexp.QuoteMeta(path)
		subpathRE = "(.*)?"
	default:
		pathRE = regexp.QuoteMeta(path)
		subpathRE = "(/.*)?"
	}

	return "^" + hostRE + portRE + pathRE + subpathRE + "$"
}

// Generates the host name to use for serving/certificate matching.
// If wildcard is set, a wildcard host name (*.<subdomain>) is generated.
func genCertificateHostName(hostname string, wildcard bool) string {
	if wildcard {
		if idx := strings.IndexRune(hostname, '.'); idx > 0 {
			return fmt.Sprintf("*.%s", hostname[idx+1:])
		}
	}

	return hostname
}

// Returns the list of endpoints for the given route's service
// action argument further processes the list e.g. shuffle
// The default action is in-order traversal of internal data structure that stores
//   the endpoints (does not change the return order if the data structure did not mutate)
func processEndpointsForAlias(alias ServiceAliasConfig, svc ServiceUnit, action string) []Endpoint {
	endpoints := endpointsForAlias(alias, svc)
	if strings.ToLower(action) == "shuffle" {
		for i := len(endpoints) - 1; i >= 0; i-- {
			rIndex := rand.Intn(i + 1)
			endpoints[i], endpoints[rIndex] = endpoints[rIndex], endpoints[i]
		}
	}
	return endpoints
}

func endpointsForAlias(alias ServiceAliasConfig, svc ServiceUnit) []Endpoint {
	if len(alias.PreferPort) == 0 {
		return svc.EndpointTable
	}
	endpoints := make([]Endpoint, 0, len(svc.EndpointTable))
	for i := range svc.EndpointTable {
		endpoint := svc.EndpointTable[i]
		if endpoint.PortName == alias.PreferPort || endpoint.Port == alias.PreferPort {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}

func (r *templateRouter) EnableRateLimiter(interval int, handlerFunc ratelimiter.HandlerFunc) {
	keyFunc := func(_ interface{}) (string, error) {
		return "templaterouter", nil
	}

	r.rateLimitedCommitFunction = ratelimiter.NewRateLimitedFunction(keyFunc, interval, handlerFunc)
	r.rateLimitedCommitFunction.RunUntil(r.rateLimitedCommitStopChannel)
	glog.V(2).Infof("Template router will coalesce reloads within %v seconds of each other", interval)
}

// secretToPem composes a PEM file at the output directory from an input private key and crt file.
func secretToPem(secPath, outName string) error {
	// The secret, when present, is mounted on /etc/pki/tls/private
	// The secret has two components crt.tls and key.tls
	// When the default cert is provided by the admin it is a pem
	//   tls.crt is the supplied pem and tls.key is the key
	//   extracted from the pem
	// When the admin does not provide a default cert, the secret
	//   is created via the service annotation. In this case
	//   tls.crt is the cert and tls.key is the key
	//   The crt and key are concatenated to form the needed pem

	var fileCrtName = filepath.Join(secPath, "tls.crt")
	var fileKeyName = filepath.Join(secPath, "tls.key")
	pemBlock, err := ioutil.ReadFile(fileCrtName)
	if err != nil {
		return err
	}
	keys, err := cmdutil.PrivateKeysFromPEM(pemBlock)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		// Try to get the key from the tls.key file
		keyBlock, err := ioutil.ReadFile(fileKeyName)
		if err != nil {
			return err
		}
		pemBlock = append(pemBlock, keyBlock...)
	}
	return ioutil.WriteFile(outName, pemBlock, 0444)
}

// writeDefaultCert ensures that the default certificate in pem format is in a file
// and the file name is set in r.defaultCertificatePath
func (r *templateRouter) writeDefaultCert() error {
	dir := filepath.Join(r.dir, certDir)
	outPath := filepath.Join(dir, fmt.Sprintf("%s.pem", defaultCertName))
	if len(r.defaultCertificate) == 0 {
		// There is no default cert. There may be a path or a secret...
		if len(r.defaultCertificatePath) != 0 {
			// Just use the provided path
			return nil
		}
		err := secretToPem(r.defaultCertificateDir, outPath)
		if err != nil {
			// no pem file, no default cert, use cert from container
			glog.V(2).Infof("Router default cert from router container")
			return nil
		}
		r.defaultCertificatePath = outPath
		return nil
	}

	// write out the default cert (pem format)
	glog.V(2).Infof("Writing default certificate to %s", dir)
	if err := r.certManager.CertificateWriter().WriteCertificate(dir, defaultCertName, []byte(r.defaultCertificate)); err != nil {
		return err
	}
	r.defaultCertificatePath = outPath
	return nil
}

func (r *templateRouter) readState() error {
	data, err := ioutil.ReadFile(filepath.Join(r.dir, routeFile))
	// TODO: rework
	if err != nil {
		r.state = make(map[string]ServiceAliasConfig)
		return nil
	}

	return json.Unmarshal(data, &r.state)
}

// Commit applies the changes made to the router configuration - persists
// the state and refresh the backend. This is all done in the background
// so that we can rate limit + coalesce multiple changes.
// Note: If this is changed FakeCommit() in fake.go should also be updated
func (r *templateRouter) Commit() {
	r.lock.Lock()

	if !r.synced {
		glog.V(4).Infof("Router state synchronized for the first time")
		r.synced = true
		r.stateChanged = true
	}

	needsCommit := r.stateChanged
	r.lock.Unlock()

	if needsCommit {
		r.rateLimitedCommitFunction.Invoke(r.rateLimitedCommitFunction)
	}
}

// commitAndReload refreshes the backend and persists the router state.
func (r *templateRouter) commitAndReload() error {
	// only state changes must be done under the lock
	if err := func() error {
		r.lock.Lock()
		defer r.lock.Unlock()

		glog.V(4).Infof("Writing the router state")
		if err := r.writeState(); err != nil {
			return err
		}

		r.stateChanged = false

		glog.V(4).Infof("Writing the router config")
		reloadStart := time.Now()
		err := r.writeConfig()
		r.metricWriteConfig.Observe(float64(time.Now().Sub(reloadStart)) / float64(time.Second))
		return err
	}(); err != nil {
		return err
	}

	for i, fn := range r.reloadCallbacks {
		glog.V(4).Infof("Calling reload function %d", i)
		fn()
	}

	glog.V(4).Infof("Reloading the router")
	reloadStart := time.Now()
	err := r.reloadRouter()
	r.metricReload.Observe(float64(time.Now().Sub(reloadStart)) / float64(time.Second))
	if err != nil {
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
	for k, cfg := range r.state {
		if err := r.writeCertificates(&cfg); err != nil {
			return fmt.Errorf("error writing certificates for %s: %v", k, err)
		}
		cfg.Status = ServiceAliasConfigStatusSaved
		r.state[k] = cfg
	}

	for path, template := range r.templates {
		file, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("error creating config file %s: %v", path, err)
		}

		data := templateData{
			WorkingDir:           r.dir,
			State:                r.state,
			ServiceUnits:         r.serviceUnits,
			DefaultCertificate:   r.defaultCertificatePath,
			DefaultDestinationCA: r.defaultDestinationCAPath,
			PeerEndpoints:        r.peerEndpoints,
			StatsUser:            r.statsUser,
			StatsPassword:        r.statsPassword,
			StatsPort:            r.statsPort,
			BindPorts:            !r.bindPortsAfterSync || r.synced,
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
		return r.certManager.WriteCertificatesForConfig(cfg)
	}
	return nil
}

// reloadRouter executes the router's reload script.
func (r *templateRouter) reloadRouter() error {
	cmd := exec.Command(r.reloadScriptPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error reloading router: %v\n%s", err, string(out))
	}
	glog.Infof("Router reloaded:\n%s", out)
	return nil
}

func (r *templateRouter) FilterNamespaces(namespaces sets.String) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if len(namespaces) == 0 {
		r.state = make(map[string]ServiceAliasConfig)
		r.serviceUnits = make(map[string]ServiceUnit)
		r.stateChanged = true
	}
	for k := range r.serviceUnits {
		// TODO: the id of a service unit should be defined inside this class, not passed in from the outside
		//   remove the leak of the abstraction when we refactor this code
		ns := strings.SplitN(k, "/", 2)[0]
		if namespaces.Has(ns) {
			continue
		}
		delete(r.serviceUnits, k)
		r.stateChanged = true
	}

	for k := range r.state {
		ns := strings.SplitN(k, "_", 2)[0]
		if namespaces.Has(ns) {
			continue
		}
		delete(r.state, k)
		r.stateChanged = true
	}
}

// CreateServiceUnit creates a new service named with the given id.
func (r *templateRouter) CreateServiceUnit(id string) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.createServiceUnitInternal(id)
}

// CreateServiceUnit creates a new service named with the given id - internal
// lockless form, caller needs to ensure lock acquisition [and release].
func (r *templateRouter) createServiceUnitInternal(id string) {
	parts := strings.SplitN(id, "/", 2)
	service := ServiceUnit{
		Name:          id,
		Hostname:      fmt.Sprintf("%s.%s.svc", parts[1], parts[0]),
		EndpointTable: []Endpoint{},
	}

	r.serviceUnits[id] = service
	r.stateChanged = true
}

// findMatchingServiceUnit finds the service with the given id - internal
// lockless form, caller needs to ensure lock acquisition [and release].
func (r *templateRouter) findMatchingServiceUnit(id string) (ServiceUnit, bool) {
	v, ok := r.serviceUnits[id]
	return v, ok
}

// FindServiceUnit finds the service with the given id.
func (r *templateRouter) FindServiceUnit(id string) (ServiceUnit, bool) {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.findMatchingServiceUnit(id)
}

// DeleteServiceUnit deletes the service with the given id.
func (r *templateRouter) DeleteServiceUnit(id string) {
	r.lock.Lock()
	defer r.lock.Unlock()

	_, ok := r.findMatchingServiceUnit(id)
	if !ok {
		return
	}

	delete(r.serviceUnits, id)
	r.stateChanged = true
}

// DeleteEndpoints deletes the endpoints for the service with the given id.
func (r *templateRouter) DeleteEndpoints(id string) {
	r.lock.Lock()
	defer r.lock.Unlock()

	service, ok := r.findMatchingServiceUnit(id)
	if !ok {
		return
	}

	service.EndpointTable = []Endpoint{}

	r.serviceUnits[id] = service

	// TODO: this is not safe (assuming that the subset of elements we are watching includes the peer endpoints)
	// should be a DNS lookup for endpoints of our service name.
	if id == r.peerEndpointsKey {
		r.peerEndpoints = []Endpoint{}
		glog.V(4).Infof("Peer endpoint table has been cleared")
	}

	r.stateChanged = true
}

// routeKey generates route key in form of Namespace_Name.  This is NOT the normal key structure of ns/name because
// it is not safe to use / in names of router config files.  This allows templates to use this key without having
// to create (or provide) a separate method
func (r *templateRouter) routeKey(route *routeapi.Route) string {
	name := controller.GetSafeRouteName(route.Name)

	// Namespace can contain dashes, so ${namespace}-${name} is not
	// unique, use an underscore instead - ${namespace}_${name} akin
	// to the way domain keys/service records use it ala
	// _$service.$proto.$name.
	// Note here that underscore (_) is not a valid DNS character and
	// is just used for the key name and not for the record/route name.
	// This also helps the use case for the key used as a router config
	// file name.
	return fmt.Sprintf("%s:%s", route.Namespace, name)
}

// createServiceAliasConfig creates a ServiceAliasConfig from a route and the router state.
// The router state is not modified in the process, so referenced ServiceUnits may not exist.
func (r *templateRouter) createServiceAliasConfig(route *routeapi.Route, routeKey string) *ServiceAliasConfig {
	wantsWildcardSupport := (route.Spec.WildcardPolicy == routeapi.WildcardPolicySubdomain)

	// The router config trumps what the route asks for/wants.
	wildcard := r.allowWildcardRoutes && wantsWildcardSupport

	// Get the service units and count the active ones (with a non-zero weight)
	serviceUnits := getServiceUnits(route)
	activeServiceUnits := 0
	for _, weight := range serviceUnits {
		if weight > 0 {
			activeServiceUnits++
		}
	}

	config := ServiceAliasConfig{
		Name:               route.Name,
		Namespace:          route.Namespace,
		Host:               route.Spec.Host,
		Path:               route.Spec.Path,
		IsWildcard:         wildcard,
		Annotations:        route.Annotations,
		ServiceUnitNames:   serviceUnits,
		ActiveServiceUnits: activeServiceUnits,
	}

	if route.Spec.Port != nil {
		config.PreferPort = route.Spec.Port.TargetPort.String()
	}

	key := fmt.Sprintf("%s %s", config.TLSTermination, routeKey)
	config.RoutingKeyName = fmt.Sprintf("%x", md5.Sum([]byte(key)))

	tls := route.Spec.TLS
	if tls != nil && len(tls.Termination) > 0 {
		config.TLSTermination = tls.Termination

		config.InsecureEdgeTerminationPolicy = tls.InsecureEdgeTerminationPolicy

		if tls.Termination == routeapi.TLSTerminationReencrypt && len(tls.DestinationCACertificate) == 0 && len(r.defaultDestinationCAPath) > 0 {
			config.VerifyServiceHostname = true
		}

		if tls.Termination != routeapi.TLSTerminationPassthrough {
			config.Certificates = make(map[string]Certificate)

			if len(tls.Certificate) > 0 {
				certKey := generateCertKey(&config)
				cert := Certificate{
					ID:         routeKey,
					Contents:   tls.Certificate,
					PrivateKey: tls.Key,
				}

				config.Certificates[certKey] = cert
			}

			if len(tls.CACertificate) > 0 {
				caCertKey := generateCACertKey(&config)
				caCert := Certificate{
					ID:       routeKey,
					Contents: tls.CACertificate,
				}

				config.Certificates[caCertKey] = caCert
			}

			if len(tls.DestinationCACertificate) > 0 {
				destCertKey := generateDestCertKey(&config)
				destCert := Certificate{
					ID:       routeKey,
					Contents: tls.DestinationCACertificate,
				}

				config.Certificates[destCertKey] = destCert
			}
		}
	}

	return &config
}

// AddRoute adds the given route to the router state if the route
// hasn't been seen before or has changed since it was last seen.
func (r *templateRouter) AddRoute(route *routeapi.Route) {
	backendKey := r.routeKey(route)

	newConfig := r.createServiceAliasConfig(route, backendKey)

	// We have to call the internal form of functions after this
	// because we are holding the state lock.
	r.lock.Lock()
	defer r.lock.Unlock()

	if existingConfig, exists := r.state[backendKey]; exists {
		if configsAreEqual(newConfig, &existingConfig) {
			return
		}

		glog.V(4).Infof("Updating route %s/%s", route.Namespace, route.Name)

		// Delete the route first, because modify is to be treated as delete+add
		r.removeRouteInternal(route)

		// TODO - clean up service units that are no longer
		// referenced.  This may be challenging if a service unit can
		// be referenced by more than one route, but the alternative
		// is having stale service units accumulate with the attendant
		// cost to router memory usage.
	} else {
		glog.V(4).Infof("Adding route %s/%s", route.Namespace, route.Name)
	}

	// Add service units referred to by the config
	for key := range newConfig.ServiceUnitNames {
		if _, ok := r.findMatchingServiceUnit(key); !ok {
			glog.V(4).Infof("Creating new frontend for key: %v", key)
			r.createServiceUnitInternal(key)
		}
	}

	r.state[backendKey] = *newConfig
	r.stateChanged = true
}

// RemoveRoute removes the given route
func (r *templateRouter) RemoveRoute(route *routeapi.Route) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.removeRouteInternal(route)
}

// removeRouteInternal removes the given route - internal
// lockless form, caller needs to ensure lock acquisition [and release].
func (r *templateRouter) removeRouteInternal(route *routeapi.Route) {
	routeKey := r.routeKey(route)
	serviceAliasConfig, ok := r.state[routeKey]
	if !ok {
		return
	}

	r.cleanUpServiceAliasConfig(&serviceAliasConfig)
	delete(r.state, routeKey)
	r.stateChanged = true
}

// AddEndpoints adds new Endpoints for the given id.
func (r *templateRouter) AddEndpoints(id string, endpoints []Endpoint) {
	r.lock.Lock()
	defer r.lock.Unlock()
	frontend, _ := r.findMatchingServiceUnit(id)

	//only make the change if there is a difference
	if reflect.DeepEqual(frontend.EndpointTable, endpoints) {
		glog.V(4).Infof("Ignoring change for %s, endpoints are the same", id)
		return
	}

	frontend.EndpointTable = endpoints
	r.serviceUnits[id] = frontend

	if id == r.peerEndpointsKey {
		r.peerEndpoints = frontend.EndpointTable
		glog.V(4).Infof("Peer endpoints updated to: %#v", r.peerEndpoints)
	}

	r.stateChanged = true
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

	// The cert is already written
	if cfg.Status == ServiceAliasConfigStatusSaved {
		return false
	}

	if cfg.Certificates == nil {
		return false
	}

	if cfg.TLSTermination == routeapi.TLSTerminationEdge || cfg.TLSTermination == routeapi.TLSTerminationReencrypt {
		if hasRequiredEdgeCerts(cfg) {
			return true
		}

		if cfg.TLSTermination == routeapi.TLSTerminationReencrypt {
			if hasReencryptDestinationCACert(cfg) {
				glog.V(4).Infof("a reencrypt route with host %s does not have an edge certificate, using default router certificate", cfg.Host)
				return true
			}
			if len(r.defaultDestinationCAPath) > 0 {
				glog.V(4).Infof("a reencrypt route with host %s does not have a destination CA, using default destination CA", cfg.Host)
				return true
			}
		}

		msg := fmt.Sprintf("a %s terminated route with host %s does not have the required certificates.  The route will still be created but no certificates will be written",
			cfg.TLSTermination, cfg.Host)
		// if a default cert is configured we'll assume it is meant to be a wildcard and only log info
		// otherwise we'll consider this a warning
		if len(r.defaultCertificatePath) > 0 {
			glog.V(4).Info(msg)
		} else {
			glog.Warning(msg)
		}
		return false
	}
	return false
}

// HasRoute indicates whether the given route is known to this router.
func (r *templateRouter) HasRoute(route *routeapi.Route) bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	key := r.routeKey(route)
	_, ok := r.state[key]
	return ok
}

// SyncedAtLeastOnce indicates whether the router has completed an initial sync.
func (r *templateRouter) SyncedAtLeastOnce() bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.synced
}

// hasRequiredEdgeCerts ensures that at least a host certificate and key are provided.
// a ca cert is not required because it may be something that is in the root cert chain
func hasRequiredEdgeCerts(cfg *ServiceAliasConfig) bool {
	hostCert, ok := cfg.Certificates[cfg.Host]
	return ok && len(hostCert.Contents) > 0 && len(hostCert.PrivateKey) > 0
}

// hasReencryptDestinationCACert checks whether a destination CA certificate has been provided.
func hasReencryptDestinationCACert(cfg *ServiceAliasConfig) bool {
	destCertKey := generateDestCertKey(cfg)
	destCACert, ok := cfg.Certificates[destCertKey]
	return ok && len(destCACert.Contents) > 0
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

// getServiceUnits returns a map of service keys to their weights.
// Weight suggests the % of traffic that a given service will receive
// compared to other services pointed to by the route.
func getServiceUnits(route *routeapi.Route) map[string]int32 {
	serviceUnits := make(map[string]int32)
	key := fmt.Sprintf("%s/%s", route.Namespace, route.Spec.To.Name)
	if route.Spec.To.Weight == nil {
		serviceUnits[key] = 0
	} else {
		serviceUnits[key] = *route.Spec.To.Weight
	}
	for _, svc := range route.Spec.AlternateBackends {
		key = fmt.Sprintf("%s/%s", route.Namespace, svc.Name)
		if svc.Weight == nil {
			serviceUnits[key] = 0
		} else {
			serviceUnits[key] = *svc.Weight
		}
	}
	return serviceUnits
}

// configsAreEqual determines whether the given service alias configs can be considered equal.
// This may be useful in determining whether a new service alias config is the same as an
// existing one or represents an update to its state.
func configsAreEqual(config1, config2 *ServiceAliasConfig) bool {
	return config1.Name == config2.Name &&
		config1.Namespace == config2.Namespace &&
		config1.Host == config2.Host &&
		config1.Path == config2.Path &&
		config1.TLSTermination == config2.TLSTermination &&
		reflect.DeepEqual(config1.Certificates, config2.Certificates) &&
		// Status isn't compared since whether certs have been written
		// to disk or not isn't relevant in determining whether a
		// route needs to be updated.
		config1.PreferPort == config2.PreferPort &&
		config1.InsecureEdgeTerminationPolicy == config2.InsecureEdgeTerminationPolicy &&
		config1.RoutingKeyName == config2.RoutingKeyName &&
		config1.IsWildcard == config2.IsWildcard &&
		config1.VerifyServiceHostname == config2.VerifyServiceHostname &&
		reflect.DeepEqual(config1.Annotations, config2.Annotations) &&
		reflect.DeepEqual(config1.ServiceUnitNames, config2.ServiceUnitNames)
}
