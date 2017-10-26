package templaterouter

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	"github.com/openshift/origin/pkg/router/controller"
)

const (
	ProtocolHTTP  = "http"
	ProtocolHTTPS = "https"
	ProtocolTLS   = "tls"

	routeFile       = "routes.json"
	certDir         = "certs"
	caCertDir       = "cacerts"
	defaultCertName = "default"

	caCertPostfix   = "_ca"
	destCertPostfix = "_pod"

	// '-' is not used because namespace can contain dashes
	// '_' is not used as this could be part of the name in the future
	// '/' is not safe to use in names of router config files
	routeKeySeparator = ":"
)

// templateRouter is a backend-agnostic router implementation
// that generates configuration files via a set of templates
// and manages the backend process with a reload script.
type templateRouter struct {
	// the directory to write router output to
	dir              string
	templates        map[string]*template.Template
	reloadScriptPath string
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
	// lock is a mutex used to prevent concurrent router state updates
	lock sync.Mutex
	// Track the start and end of router reloads
	lastReloadStart time.Time
	lastReloadEnd   time.Time
	// commitReqTime is nil if no commit is needed, otherwise it is the time the commit was requested
	commitReqTime *time.Time
	// commitRunning indicates whether the commitFunc is actively running
	commitRunning bool
	// commitTimer is the timer we use to make callbacks to the delayed commits
	commitTimer *time.Timer
	// reloadInterval is the minimum time between the starts of reloads
	reloadInterval time.Duration
	// reloadGap is the minimum gap between the end of a reload and the start of the next
	reloadGap time.Duration
	// reloadEventWait is the duration to wait after an event before triggering a reload in case other events come from the same change (should be short... milliseconds)
	reloadEventWait time.Duration
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
	// commitFunc is the commit (persist state + refresh the backend) function.  This is only to be used for our test hooks
	commitFunc CommitFunc
}

// templateRouterCfg holds all configuration items required to initialize the template router
type templateRouterCfg struct {
	dir                      string
	templates                map[string]*template.Template
	reloadScriptPath         string
	reloadInterval           time.Duration
	reloadGap                time.Duration
	reloadEventWait          time.Duration
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

		commitReqTime: nil,
		commitRunning: false,
		stateChanged:  true,

		reloadInterval:  cfg.reloadInterval,
		reloadGap:       cfg.reloadGap,
		reloadEventWait: cfg.reloadEventWait,

		metricReload:      metricsReload,
		metricWriteConfig: metricWriteConfig,
	}

	router.SetCommitFunc(func() error {
		return router.commitAndReload()
	})

	glog.V(2).Infof("Template router will coalesce reloads within %s seconds of the last restart start time, within %s seconds of the last restart end time, and wait %s seconds after the first event", router.reloadInterval.String(), router.reloadGap.String(), router.reloadEventWait.String())

	if err := router.writeDefaultCert(); err != nil {
		return nil, err
	}

	glog.V(4).Infof("Reading persisted state")
	if err := router.readState(); err != nil {
		return nil, err
	}

	// Do an immediate commit so that we can reply to health checks before the state syncs
	glog.V(4).Infof("Committing without a sync so that the router replies to health checks")
	router.realCommit(false)

	return router, nil
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

// Define a type for the pluggable commit function
type CommitFunc func() error

// This should ONLY be used for external testing hooks
func (r *templateRouter) SetCommitFunc(commitFunc CommitFunc) {
	r.commitFunc = commitFunc
}

// Commit calls the realCommit worker with a flag saying that it has
// synced.  This should be called only when the router state is
// consistent.  realCommit() below can be called on initial router
// load to load the haproxy without a full state sync.
func (r *templateRouter) Commit() {
	r.realCommit(true)
}

// realCommit applies the changes made to the router configuration - persists
// the state and refresh the backend. This is all done in the background
// so that we can rate limit + coalesce multiple changes.
//
// The hasSynced argument specifies whether the state is fully synchronized.
//
// Note: If this is changed FakeCommit() in fake.go should also be updated
func (r *templateRouter) realCommit(hasSynced bool) {
	r.lock.Lock()

	if hasSynced {
		// Only update our sync state if we've really synced.  We want
		// to let the router start early so that it can return health
		// while the state synchronizes, but before the main ports are
		// bound
		if !r.synced {
			glog.V(4).Infof("Router state synchronized for the first time, allowing rapid reload")
			r.synced = true
			r.stateChanged = true

			// Clean out the last reload variables so the reload from the first sync doesn't have to wait
			r.lastReloadStart = time.Time{}
			r.lastReloadEnd = time.Time{}
		}
	}

	if r.stateChanged {
		// If the state changed, then we need to commit
		glog.V(8).Infof("Commit called and the state has changed, could reload")

		if r.commitReqTime == nil {
			// There is no scheduled commit worker, so set the time we started and
			// invoke the worker code.  It will decide if it can run now, or if it
			// needs to schedule a callback.
			//
			// We need to track the earliest commit time so we can do burst supression.
			now := time.Now()
			r.commitReqTime = &now
			r.lock.Unlock()

			glog.V(8).Infof("No scheduled reload, calling the worker (curtime: %v)", now)

			r.commitWorker()
			return
		}

		glog.V(8).Infof("There is already a scheduled reload (for %v), skipping the worker", r.commitReqTime)
	}

	r.lock.Unlock()
}

// timeUntilNextAction() works out when we can next reload based on the current time, the last action time, and the minimum allowed gap between the two.
// In order to be allowed to reload immediately, the last action + minumum gap must be < the current time.
// If we can reload, then it returns a zero duration.
// If we can't reload, then it reuturns a duration to wait for before a reload would be allowed.
func timeUntilNextAction(now time.Time, lastActionTime time.Time, minimumActionGap time.Duration) (nextReload time.Duration) {
	if sinceLastAction := now.Sub(lastActionTime); sinceLastAction < minimumActionGap {
		return minimumActionGap - sinceLastAction
	}

	return 0 * time.Second
}

func (r *templateRouter) commitWorker() {
	r.lock.Lock()

	glog.V(8).Infof("CommitWorker called")

	if r.commitRunning {
		// We don't need to do anything else... there's a commit in progress, and when it is done it will re-call this function at which point the work will then happen
		glog.V(8).Infof("There was already a commit running (%v) returning from the worker", r.commitRunning)
		r.lock.Unlock()
		return
	}

	if r.commitReqTime == nil {
		// There's no commit queued so we have nothing to do.  We should only get here when
		// the function is re-called after a reload
		glog.V(8).Infof("No commit requested time, so there's no queued commit.  Nothing to do.")
		r.lock.Unlock()
		return
	}

	// There is no commit running, let's see if we should run yet, or schedule a callback
	var untilNextCallback time.Duration
	now := time.Now()

	getNextReload := func(potentialDuration time.Duration) time.Duration {
		// Use the largest of the potential durations so we only come back when the checks will allow it to run
		if potentialDuration > untilNextCallback {
			return potentialDuration
		}

		return untilNextCallback
	}

	untilNextCallback = getNextReload(timeUntilNextAction(now, r.lastReloadStart, r.reloadInterval))
	untilNextCallback = getNextReload(timeUntilNextAction(now, r.lastReloadEnd, r.reloadGap))
	untilNextCallback = getNextReload(timeUntilNextAction(now, *r.commitReqTime, r.reloadEventWait))

	if untilNextCallback > 0 {
		// We want to reload... but can't yet because some window is not satisfied
		if r.commitTimer == nil {
			r.commitTimer = time.AfterFunc(untilNextCallback, r.commitWorker)
		} else {
			// While we are resetting the timer, it should have fired and be stopped.
			// The first time the worker is called it will know the precise duration
			// until when a run would be valid and has scheduled a timer for that point
			r.commitTimer.Reset(untilNextCallback)
		}

		glog.V(8).Infof("Can't reload the router yet, need to delay %s, callback scheduled", untilNextCallback.String())

		r.lock.Unlock()
		return
	}

	// Otherwise we can reload immediately... let's do it!
	glog.V(8).Infof("Calling the router commit function (for req time %v)", r.commitReqTime)
	r.commitRunning = true
	r.commitReqTime = nil
	r.lock.Unlock()

	if err := func() error {
		defer func() {
			r.lock.Lock()
			r.commitRunning = false
			r.lock.Unlock()
		}()

		return r.commitFunc()
	}(); err != nil {
		utilruntime.HandleError(err)
	}

	// Re-call the commit in case there is work waiting that came in while we were working
	// we want to call the top level commit in case the state has not changed
	glog.V(8).Infof("Re-Calling the worker after a reload in case work came in")
	r.commitWorker()
}

// commitAndReload refreshes the backend and persists the router state.
// This is the default implementation of the commit function, it can be replaced by the FakeReloadHandler() for testing purposes.
// If you change any state handling here make sure you keep that in sync.
//
// Note: Only one commitAndReload can be in progress at a time, but the Commit function takes care of
// ensuring that only one commit function is called.
func (r *templateRouter) commitAndReload() error {

	if err := func() error {
		// Only state reads and changes must be done under the lock, the reload itself must not be done under the lock
		r.lock.Lock()
		defer r.lock.Unlock()

		glog.V(4).Infof("Writing the router state")
		if err := r.writeState(); err != nil {
			return err
		}

		r.stateChanged = false
		r.lastReloadStart = time.Now()

		glog.V(4).Infof("Writing the router config")
		err := r.writeConfig()
		r.metricWriteConfig.Observe(float64(time.Now().Sub(r.lastReloadStart)) / float64(time.Second))
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
	reloadEnd := time.Now()
	r.metricReload.Observe(float64(reloadEnd.Sub(reloadStart)) / float64(time.Second))
	if err != nil {
		return err
	}

	r.lock.Lock()
	r.lastReloadEnd = reloadEnd
	r.lock.Unlock()

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
// Must be called while holding r.lock
func (r *templateRouter) writeConfig() error {
	//write out any certificate files that don't exist
	for k, cfg := range r.state {
		if err := r.writeCertificates(&cfg); err != nil {
			return fmt.Errorf("error writing certificates for %s: %v", k, err)
		}

		// calculate the server weight for the endpoints in each service
		// called here to make sure we have the actual number of endpoints.
		cfg.ServiceUnitNames = r.calculateServiceWeights(cfg.ServiceUnits)

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
		ns, _ := getPartsFromEndpointsKey(k)
		if namespaces.Has(ns) {
			continue
		}
		delete(r.serviceUnits, k)
		r.stateChanged = true
	}

	for k := range r.state {
		ns, _ := getPartsFromRouteKey(k)
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
	namespace, name := getPartsFromEndpointsKey(id)
	service := ServiceUnit{
		Name:          id,
		Hostname:      fmt.Sprintf("%s.%s.svc", name, namespace),
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

// routeKey generates route key. This allows templates to use this key without having to create a separate method
func routeKey(route *routeapi.Route) string {
	return routeKeyFromParts(route.Namespace, controller.GetSafeRouteName(route.Name))
}

func routeKeyFromParts(namespace, name string) string {
	return fmt.Sprintf("%s%s%s", namespace, routeKeySeparator, name)
}

func getPartsFromRouteKey(key string) (string, string) {
	tokens := strings.SplitN(key, routeKeySeparator, 2)
	if len(tokens) != 2 {
		glog.Errorf("Expected separator %q not found in route key %q", routeKeySeparator, key)
	}
	namespace := tokens[0]
	name := tokens[1]
	return namespace, name
}

// createServiceAliasConfig creates a ServiceAliasConfig from a route and the router state.
// The router state is not modified in the process, so referenced ServiceUnits may not exist.
func (r *templateRouter) createServiceAliasConfig(route *routeapi.Route, backendKey string) *ServiceAliasConfig {
	wantsWildcardSupport := (route.Spec.WildcardPolicy == routeapi.WildcardPolicySubdomain)

	// The router config trumps what the route asks for/wants.
	wildcard := r.allowWildcardRoutes && wantsWildcardSupport

	// Get the service weights from each service in the route. Count the active
	// ones (with a non-zero weight)
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
		ServiceUnits:       serviceUnits,
		ActiveServiceUnits: activeServiceUnits,
	}

	if route.Spec.Port != nil {
		config.PreferPort = route.Spec.Port.TargetPort.String()
	}

	key := fmt.Sprintf("%s %s", config.TLSTermination, backendKey)
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
					ID:         backendKey,
					Contents:   tls.Certificate,
					PrivateKey: tls.Key,
				}

				config.Certificates[certKey] = cert
			}

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

	return &config
}

// AddRoute adds the given route to the router state if the route
// hasn't been seen before or has changed since it was last seen.
func (r *templateRouter) AddRoute(route *routeapi.Route) {
	backendKey := routeKey(route)

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
	for key := range newConfig.ServiceUnits {
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
	backendKey := routeKey(route)
	serviceAliasConfig, ok := r.state[backendKey]
	if !ok {
		return
	}

	r.cleanUpServiceAliasConfig(&serviceAliasConfig)
	delete(r.state, backendKey)
	r.stateChanged = true
}

// numberOfEndpoints returns the number of endpoints
// Must be called while holding r.lock
func (r *templateRouter) numberOfEndpoints(id string) int32 {
	var eps = 0
	svc, ok := r.findMatchingServiceUnit(id)
	if ok && len(svc.EndpointTable) > eps {
		eps = len(svc.EndpointTable)
	}
	return int32(eps)
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
	key := routeKey(route)
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
// The requests are loadbalanced among the services referenced by the route.
// The weight (0-256, default 1) sets the relative proportions each
// service gets (weight/sum_of_weights) fraction of the requests.
// Default when service weight is omitted is 1.
// if weight < 0 or > 256 set to 0.
// When the weight is 0 no traffic goes to the service. If they are
// all 0 the request is returned with 503 response.
func getServiceUnits(route *routeapi.Route) map[string]int32 {
	serviceUnits := make(map[string]int32)

	// get the weight and number of endpoints for each service
	key := endpointsKeyFromParts(route.Namespace, route.Spec.To.Name)
	serviceUnits[key] = 0
	if route.Spec.To.Weight != nil {
		serviceUnits[key] = int32(*route.Spec.To.Weight)
	}
	if serviceUnits[key] < 0 || serviceUnits[key] > 256 {
		serviceUnits[key] = 0
	}

	for _, svc := range route.Spec.AlternateBackends {
		key = endpointsKeyFromParts(route.Namespace, svc.Name)
		serviceUnits[key] = 0
		if svc.Weight != nil {
			serviceUnits[key] = int32(*svc.Weight)
		}
		if serviceUnits[key] < 0 || serviceUnits[key] > 256 {
			serviceUnits[key] = 0
		}
	}

	return serviceUnits
}

// calculateServiceWeights returns a map of service keys to their weights.
// Each service gets (weight/sum_of_weights) fraction of the requests.
// For each service, the requests are distributed among the endpoints.
// Each endpoint gets weight/numberOfEndpoints portion of the requests.
// The largest weight per endpoint is scaled to 256 to permit better
// percision results.  The remainder are scaled using the same scale factor.
// Inaccuracies occur when converting float32 to int32 and when the scaled
// weight per endpoint is less than 1.0, the minimum.
// The above assumes roundRobin scheduling.
func (r *templateRouter) calculateServiceWeights(serviceUnits map[string]int32) map[string]int32 {
	serviceUnitNames := make(map[string]int32)
	// portion of service weight for each endpoint
	epWeight := make(map[string]float32)
	// maximum endpoint weight
	var maxEpWeight float32 = 0.0

	// distribute service weight over the service's endpoints
	// to get weight per endpoint
	for key, units := range serviceUnits {
		numEp := r.numberOfEndpoints(key)
		if numEp > 0 {
			epWeight[key] = float32(units) / float32(numEp)
		}
		if epWeight[key] > maxEpWeight {
			maxEpWeight = epWeight[key]
		}
	}

	// Scale the weights to near the maximum (256).
	// This improves precision when scaling for the endpoints
	var scaleWeight float32 = 0.0
	if maxEpWeight > 0.0 {
		scaleWeight = 256.0 / maxEpWeight
	}

	// The weight assigned to the service is distributed among the endpoints
	// for example the if we have two services "A" with weight 20 and 2 endpoints
	// and "B" with  weight 10 and 4 endpoints the ultimate weights on
	// endpoints would work out as:
	// service "A" weight per endpoint 10.0
	// service "B" weight per endpoint 2.5
	// maximum endpoint weight is 10.0 so scale is 25.6
	// service "A" scaled endpoint weight 256.0 truncated to 256
	// service "B" scaled endpoint weight 64.0 truncated to 64
	// So, all service "A" endpoints get 256 and all service "B" endpoints get 64

	for key, weight := range epWeight {
		serviceUnitNames[key] = int32(weight * scaleWeight)
		if weight > 0.0 && serviceUnitNames[key] < 1 {
			serviceUnitNames[key] = 1
			numEp := r.numberOfEndpoints(key)
			glog.V(4).Infof("%s: WARNING: Too many endpoints to achieve desired weight for route. Service can have %d but has %d endpoints", key, int32(weight*float32(numEp)), numEp)
		}
		glog.Infof("%s: weight %d  %f  %d", key, serviceUnits[key], weight, serviceUnitNames[key]) // PHIL debug
	}

	return serviceUnitNames
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
		reflect.DeepEqual(config1.ServiceUnits, config2.ServiceUnits)
}
