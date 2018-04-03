package ingress

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/json"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	extensionsinformers "k8s.io/client-go/informers/extensions/v1beta1"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	extensionslisters "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	routeinformers "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	routelisters "github.com/openshift/client-go/route/listers/route/v1"
)

// Controller ensures that zero or more routes exist to match any supported ingress. The
// controller creates a controller owner reference from the route to the parent ingress,
// allowing users to orphan their ingress. All owned routes have specific spec fields
// managed (those attributes present on the ingress), while any other fields may be
// modified by the user.
//
// Invariants:
//
// 1. For every ingress path rule with a non-empty backend statement, a route should
//    exist that points to that backend.
// 2. For every TLS hostname that has a corresponding path rule and points to a secret
//    that exists, a route should exist with a valid TLS config from that secret.
// 3. For every service referenced by the ingress path rule, the route should have
//    a target port based on the service.
// 4. A route owned by an ingress that is not described by any of the three invariants
//    above should be deleted.
//
// The controller also relies on the use of expectations to remind itself whether there
// are route creations it has not yet observed, which prevents the controller from
// creating more objects than it needs. The expectations are reset when the ingress
// object is modified. It is possible that expectations could leak if an ingress is
// deleted and its deletion is not observed by the cache, but such leaks are only expected
// if there is a bug in the informer cache which must be fixed anyway.
//
// Unsupported attributes:
//
// * the ingress class attribute
// * nginx annotations
// * the empty backend
// * paths with empty backends
// * creating a dynamic route spec.host
//
type Controller struct {
	eventRecorder record.EventRecorder

	client routeclient.RoutesGetter

	ingressLister extensionslisters.IngressLister
	secretLister  corelisters.SecretLister
	routeLister   routelisters.RouteLister
	serviceLister corelisters.ServiceLister

	// syncs are the items that must return true before the queue can be processed
	syncs []cache.InformerSynced

	// queue is the list of namespace keys that must be synced.
	queue workqueue.RateLimitingInterface

	// expectations track upcoming route creations that we have not yet observed
	expectations *expectations
	// expectationDelay controls how long the controller waits to observe its
	// own creates. Exposed only for testing.
	expectationDelay time.Duration
}

// expectations track an upcoming change to a named resource related
// to an ingress. This is a thread safe object but callers assume
// responsibility for ensuring expectations do not leak.
type expectations struct {
	lock   sync.Mutex
	expect map[queueKey]sets.String
}

// newExpectations returns a tracking object for upcoming events
// that the controller may expect to happen.
func newExpectations() *expectations {
	return &expectations{
		expect: make(map[queueKey]sets.String),
	}
}

// Expect that an event will happen in the future for the given ingress
// and a named resource related to that ingress.
func (e *expectations) Expect(namespace, ingressName, name string) {
	e.lock.Lock()
	defer e.lock.Unlock()
	key := queueKey{namespace: namespace, name: ingressName}
	set, ok := e.expect[key]
	if !ok {
		set = sets.NewString()
		e.expect[key] = set
	}
	set.Insert(name)
}

// Satisfied clears the expectation for the given resource name on an
// ingress.
func (e *expectations) Satisfied(namespace, ingressName, name string) {
	e.lock.Lock()
	defer e.lock.Unlock()
	key := queueKey{namespace: namespace, name: ingressName}
	set := e.expect[key]
	set.Delete(name)
	if set.Len() == 0 {
		delete(e.expect, key)
	}
}

// Expecting returns true if the provided ingress is still waiting to
// see changes.
func (e *expectations) Expecting(namespace, ingressName string) bool {
	e.lock.Lock()
	defer e.lock.Unlock()
	key := queueKey{namespace: namespace, name: ingressName}
	return e.expect[key].Len() > 0
}

// Clear indicates that all expectations for the given ingress should
// be cleared.
func (e *expectations) Clear(namespace, ingressName string) {
	e.lock.Lock()
	defer e.lock.Unlock()
	key := queueKey{namespace: namespace, name: ingressName}
	delete(e.expect, key)
}

type queueKey struct {
	namespace string
	name      string
}

// NewController instantiates a Controller
func NewController(eventsClient kv1core.EventsGetter, client routeclient.RoutesGetter, ingresses extensionsinformers.IngressInformer, secrets coreinformers.SecretInformer, services coreinformers.ServiceInformer, routes routeinformers.RouteInformer) *Controller {
	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(glog.Infof)
	// TODO: remove the wrapper when every clients have moved to use the clientset.
	broadcaster.StartRecordingToSink(&kv1core.EventSinkImpl{Interface: eventsClient.Events("")})
	recorder := broadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "ingress-to-route-controller"})

	c := &Controller{
		eventRecorder: recorder,
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ingress-to-route"),

		expectations:     newExpectations(),
		expectationDelay: 2 * time.Second,

		client: client,

		ingressLister: ingresses.Lister(),
		secretLister:  secrets.Lister(),
		routeLister:   routes.Lister(),
		serviceLister: services.Lister(),

		syncs: []cache.InformerSynced{
			ingresses.Informer().HasSynced,
			secrets.Informer().HasSynced,
			routes.Informer().HasSynced,
			services.Informer().HasSynced,
		},
	}

	// any change to a secret of type TLS in the namespace
	secrets.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *v1.Secret:
				return t.Type == v1.SecretTypeTLS
			}
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.processNamespace,
			DeleteFunc: c.processNamespace,
			UpdateFunc: func(oldObj, newObj interface{}) {
				c.processNamespace(newObj)
			},
		},
	})

	// any change to a service in the namespace
	services.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.processNamespace,
		DeleteFunc: c.processNamespace,
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.processNamespace(newObj)
		},
	})

	// any change to a route that has the controller relationship to an Ingress
	routes.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *routev1.Route:
				_, ok := hasIngressOwnerRef(t.OwnerReferences)
				return ok
			}
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.processRoute,
			DeleteFunc: c.processRoute,
			UpdateFunc: func(oldObj, newObj interface{}) {
				c.processRoute(newObj)
			},
		},
	})

	// changes to ingresses
	ingresses.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.processIngress,
		DeleteFunc: c.processIngress,
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.processIngress(newObj)
		},
	})

	return c
}

func (c *Controller) processNamespace(obj interface{}) {
	switch t := obj.(type) {
	case metav1.Object:
		ns := t.GetNamespace()
		if len(ns) == 0 {
			utilruntime.HandleError(fmt.Errorf("object %T has no namespace", obj))
			return
		}
		c.queue.Add(queueKey{namespace: ns})
	default:
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %T", obj))
	}
}

func (c *Controller) processRoute(obj interface{}) {
	switch t := obj.(type) {
	case *routev1.Route:
		ingressName, ok := hasIngressOwnerRef(t.OwnerReferences)
		if !ok {
			return
		}
		c.expectations.Satisfied(t.Namespace, ingressName, t.Name)
		c.queue.Add(queueKey{namespace: t.Namespace, name: ingressName})
	default:
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %T", obj))
	}
}

func (c *Controller) processIngress(obj interface{}) {
	switch t := obj.(type) {
	case *extensionsv1beta1.Ingress:
		// when we see a change to an ingress, reset our expectations
		// this also allows periodic purging of the expectation list in the event
		// we miss one or more events.
		c.expectations.Clear(t.Namespace, t.Name)
		c.queue.Add(queueKey{namespace: t.Namespace, name: t.Name})
	default:
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %T", obj))
	}
}

// Run begins watching and syncing.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting controller")

	if !cache.WaitForCacheSync(stopCh, c.syncs...) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
	glog.Infof("Shutting down controller")
}

func (c *Controller) worker() {
	for c.processNext() {
	}
	glog.V(4).Infof("Worker stopped")
}

func (c *Controller) processNext() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	glog.V(5).Infof("processing %v begin", key)
	err := c.sync(key.(queueKey))
	c.handleNamespaceErr(err, key)
	glog.V(5).Infof("processing %v end", key)

	return true
}

func (c *Controller) handleNamespaceErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	glog.V(4).Infof("Error syncing %v: %v", key, err)
	c.queue.AddRateLimited(key)
}

func (c *Controller) sync(key queueKey) error {
	// sync all ingresses in the namespace
	if len(key.name) == 0 {
		ingresses, err := c.ingressLister.Ingresses(key.namespace).List(labels.Everything())
		if err != nil {
			return err
		}
		for _, ingress := range ingresses {
			c.queue.Add(queueKey{namespace: ingress.Namespace, name: ingress.Name})
		}
		return nil
	}
	// if we are waiting to observe the result of route creations, simply delay
	if c.expectations.Expecting(key.namespace, key.name) {
		c.queue.AddAfter(key, c.expectationDelay)
		glog.V(5).Infof("Ingress %s/%s has unsatisfied expectations", key.namespace, key.name)
		return nil
	}

	ingress, err := c.ingressLister.Ingresses(key.namespace).Get(key.name)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	// find all matching routes
	routes, err := c.routeLister.Routes(key.namespace).List(labels.Everything())
	if err != nil {
		return err
	}
	old := routes[:0]
	for _, route := range routes {
		ingressName, ok := hasIngressOwnerRef(route.OwnerReferences)
		if !ok || ingressName != ingress.Name {
			continue
		}
		old = append(old, route)
	}

	// walk the ingress and identify whether any of the child routes need to be updated, deleted,
	// or created, as efficiently as possible.
	var creates, updates []*routev1.Route
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		if len(rule.Host) == 0 {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if len(path.Backend.ServiceName) == 0 {
				continue
			}

			var existing *routev1.Route
			old, existing = splitForPathAndHost(old, rule.Host, path.Path)
			if existing == nil {
				if r := newRouteForIngress(ingress, &rule, &path, c.secretLister, c.serviceLister); r != nil {
					creates = append(creates, r)
				}
				continue
			}

			if routeMatchesIngress(existing, ingress, &rule, &path, c.secretLister, c.serviceLister) {
				continue
			}

			if r := newRouteForIngress(ingress, &rule, &path, c.secretLister, c.serviceLister); r != nil {
				// merge the relevant spec pieces
				preserveRouteAttributesFromExisting(r, existing)
				updates = append(updates, r)
			} else {
				// the route cannot be fully calculated, delete it
				old = append(old, existing)
			}
		}
	}

	var errs []error

	// add the new routes
	for _, route := range creates {
		if err := createRouteWithName(c.client, ingress, route, c.expectations); err != nil {
			errs = append(errs, err)
		}
	}

	// update any existing routes in place
	for _, route := range updates {
		data, err := json.Marshal(&route.Spec)
		if err != nil {
			return err
		}
		data = []byte(fmt.Sprintf(`[{"op":"replace","path":"/spec","value":%s}]`, data))
		_, err = c.client.Routes(route.Namespace).Patch(route.Name, types.JSONPatchType, data)
		if err != nil {
			errs = append(errs, err)
		}
	}

	// purge any previously managed routes
	for _, route := range old {
		if err := c.client.Routes(route.Namespace).Delete(route.Name, nil); err != nil && !errors.IsNotFound(err) {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return utilerrors.NewAggregate(errs)
	}
	return nil
}

func hasIngressOwnerRef(owners []metav1.OwnerReference) (string, bool) {
	for _, ref := range owners {
		if ref.Kind != "Ingress" || ref.APIVersion != "extensions/v1beta1" || ref.Controller == nil || !*ref.Controller {
			continue
		}
		return ref.Name, true
	}
	return "", false
}

func newRouteForIngress(
	ingress *extensionsv1beta1.Ingress,
	rule *extensionsv1beta1.IngressRule,
	path *extensionsv1beta1.HTTPIngressPath,
	secretLister corelisters.SecretLister,
	serviceLister corelisters.ServiceLister,
) *routev1.Route {
	var tlsConfig *routev1.TLSConfig
	if name, ok := referencesSecret(ingress, rule.Host); ok {
		secret, err := secretLister.Secrets(ingress.Namespace).Get(name)
		if err != nil {
			// secret doesn't exist yet, wait
			return nil
		}
		if secret.Type != v1.SecretTypeTLS {
			// secret is the wrong type
			return nil
		}
		if _, ok := secret.Data[v1.TLSCertKey]; !ok {
			return nil
		}
		if _, ok := secret.Data[v1.TLSPrivateKeyKey]; !ok {
			return nil
		}
		tlsConfig = &routev1.TLSConfig{
			Termination: routev1.TLSTerminationEdge,
			Certificate: string(secret.Data[v1.TLSCertKey]),
			Key:         string(secret.Data[v1.TLSPrivateKeyKey]),
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
		}
	}

	targetPort := targetPortForService(ingress.Namespace, path, serviceLister)
	if targetPort == nil {
		// no valid target port
		return nil
	}

	t := true
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: ingress.Name + "-",
			Namespace:    ingress.Namespace,
			Labels:       ingress.Labels,
			Annotations:  ingress.Annotations,
			OwnerReferences: []metav1.OwnerReference{
				{APIVersion: "extensions/v1beta1", Kind: "Ingress", Controller: &t, Name: ingress.Name, UID: ingress.UID},
			},
		},
		Spec: routev1.RouteSpec{
			Host: rule.Host,
			Path: path.Path,
			To: routev1.RouteTargetReference{
				Name: path.Backend.ServiceName,
			},
			Port: &routev1.RoutePort{
				TargetPort: *targetPort,
			},
			TLS: tlsConfig,
		},
	}
}

func preserveRouteAttributesFromExisting(r, existing *routev1.Route) {
	r.Name = existing.Name
	r.GenerateName = ""
	r.Spec.To.Weight = existing.Spec.To.Weight
	if r.Spec.TLS != nil && existing.Spec.TLS != nil {
		r.Spec.TLS.CACertificate = existing.Spec.TLS.CACertificate
		r.Spec.TLS.DestinationCACertificate = existing.Spec.TLS.DestinationCACertificate
		r.Spec.TLS.InsecureEdgeTerminationPolicy = existing.Spec.TLS.InsecureEdgeTerminationPolicy
	}
}

func routeMatchesIngress(
	route *routev1.Route,
	ingress *extensionsv1beta1.Ingress,
	rule *extensionsv1beta1.IngressRule,
	path *extensionsv1beta1.HTTPIngressPath,
	secretLister corelisters.SecretLister,
	serviceLister corelisters.ServiceLister,
) bool {
	match := route.Spec.Host == rule.Host &&
		route.Spec.Path == path.Path &&
		route.Spec.To.Name == path.Backend.ServiceName &&
		route.Spec.Port != nil &&
		route.Spec.WildcardPolicy == routev1.WildcardPolicyNone &&
		len(route.Spec.AlternateBackends) == 0
	if !match {
		return false
	}

	targetPort := targetPortForService(ingress.Namespace, path, serviceLister)
	if targetPort == nil || *targetPort != route.Spec.Port.TargetPort {
		// not valid
		return false
	}

	var secret *v1.Secret
	if name, ok := referencesSecret(ingress, rule.Host); ok {
		secret, _ = secretLister.Secrets(ingress.Namespace).Get(name)
		if secret == nil {
			return false
		}
	}
	if !secretMatchesRoute(secret, route.Spec.TLS) {
		return false
	}
	return true
}

func targetPortForService(namespace string, path *extensionsv1beta1.HTTPIngressPath, serviceLister corelisters.ServiceLister) *intstr.IntOrString {
	service, err := serviceLister.Services(namespace).Get(path.Backend.ServiceName)
	if err != nil {
		// service doesn't exist yet, wait
		return nil
	}
	if path.Backend.ServicePort.Type == intstr.String {
		expect := path.Backend.ServicePort.StrVal
		for _, port := range service.Spec.Ports {
			if port.Name == expect {
				return &port.TargetPort
			}
		}
	} else {
		for _, port := range service.Spec.Ports {
			expect := path.Backend.ServicePort.IntVal
			if port.Port == expect {
				return &port.TargetPort
			}
		}
	}
	return nil
}

func secretMatchesRoute(secret *v1.Secret, tlsConfig *routev1.TLSConfig) bool {
	if secret == nil {
		return tlsConfig == nil
	}
	if secret.Type != v1.SecretTypeTLS {
		return tlsConfig == nil
	}
	if _, ok := secret.Data[v1.TLSCertKey]; !ok {
		return false
	}
	if _, ok := secret.Data[v1.TLSPrivateKeyKey]; !ok {
		return false
	}
	if tlsConfig == nil {
		return false
	}
	return tlsConfig.Termination == routev1.TLSTerminationEdge &&
		tlsConfig.Certificate == string(secret.Data[v1.TLSCertKey]) &&
		tlsConfig.Key == string(secret.Data[v1.TLSPrivateKeyKey])
}

func splitForPathAndHost(routes []*routev1.Route, host, path string) ([]*routev1.Route, *routev1.Route) {
	for i, route := range routes {
		if route.Spec.Host == host && route.Spec.Path == path {
			last := len(routes) - 1
			routes[i], routes[last] = routes[last], route
			return routes[:last], route
		}
	}
	return routes, nil
}

func referencesSecret(ingress *extensionsv1beta1.Ingress, host string) (string, bool) {
	for _, tls := range ingress.Spec.TLS {
		for _, tlsHost := range tls.Hosts {
			if tlsHost == host {
				return tls.SecretName, true
			}
		}
	}
	return "", false
}

// createRouteWithName performs client side name generation so we can set a predictable expectation.
// If we fail multiple times in a row we will return an error.
// TODO: future optimization, check the local cache for the name first
func createRouteWithName(client routeclient.RoutesGetter, ingress *extensionsv1beta1.Ingress, route *routev1.Route, expect *expectations) error {
	base := route.GenerateName
	var lastErr error
	// only retry a limited number of times
	for i := 0; i < 3; i++ {
		if len(base) > 0 {
			route.GenerateName = ""
			route.Name = generateRouteName(base)
		}

		// Set the expectation before we talk to the server in order to
		// prevent racing with the route cache.
		expect.Expect(ingress.Namespace, ingress.Name, route.Name)

		_, err := client.Routes(route.Namespace).Create(route)
		if err == nil {
			return nil
		}

		// We either collided with another randomly generated name, or another
		// error between us and the server prevented observing the success
		// of the result. In either case we are not expecting a new route. This
		// is safe because expectations are an optimization to avoid churn rather
		// than to prevent true duplicate creation.
		expect.Satisfied(ingress.Namespace, ingress.Name, route.Name)

		// if we aren't generating names (or if we got any other type of error)
		// return right away
		if len(base) == 0 || !errors.IsAlreadyExists(err) {
			return err
		}
		lastErr = err
	}
	return lastErr
}

const (
	maxNameLength          = 63
	randomLength           = 5
	maxGeneratedNameLength = maxNameLength - randomLength
)

func generateRouteName(base string) string {
	if len(base) > maxGeneratedNameLength {
		base = base[:maxGeneratedNameLength]
	}
	return fmt.Sprintf("%s%s", base, utilrand.String(randomLength))
}
