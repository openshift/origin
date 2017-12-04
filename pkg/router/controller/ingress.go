package controller

import (
	"crypto/md5"
	"fmt"
	"strings"
	"sync"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/extensions"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

// ingressRouteEvents maps an ingress key to route events
// generated from the ingress.  It is intended to be the
// return type used by the ingress and secret event
// translation methods.
type ingressRouteEvents struct {
	ingressKey  string
	routeEvents []routeEvent
}

type routeEvent struct {
	eventType watch.EventType
	route     *routeapi.Route
}

type ingressRouteMap map[string]*routeapi.Route

type ingressMap map[string]*extensions.Ingress

// IngressTranslator converts secret and ingress events into route events.
//
// - Caches ingresses to enable:
//   - Identification of which secrets are referenced by ingresses
//   - Generation of route events in response to a secret event
//   - Route deletion when an ingress rule is removed
//
// - Caches secrets to minimize the lookups required to generate route events from an ingress
//   - Secrets will be read into the cache via Get() the first time they are referenced
//     - Only secrets referenced by an ingress cached by the router will themselves be cached
//   - Secrets will be updated by calls to TranslateSecretEvent
type IngressTranslator struct {
	// Synchronize access to the maps
	lock sync.Mutex

	// Tracks ingresses seen by the router.  Key is [namespace]/[ingress name].
	ingressMap ingressMap

	// Caches tls from secrets referenced by ingresses.  Key is
	// [namespace]/[secret name].  Entries should be added to the map
	// for all referenced secrets - not just those that could be
	// retrieved and read - to ensure that a retrieval failure can be
	// incurred only once per secret rather than once per reference.
	tlsMap map[string]*referencedTLS

	// Enables ingress handling to lookup referenced secrets
	client kcoreclient.SecretsGetter

	// If non-nil, ingresses will only be translated into routes if
	// they are in an allowed namespace
	allowedNamespaces sets.String
}

// referencedTLS records the ingress keys ([namespace]/[name]) of ingresses that reference a
// given tls configuration.  Intended for use in a map indexed by secret key.
type referencedTLS struct {
	ingressKeys sets.String
	tls         *cachedTLS
}

// cachedTLS caches the cert and key data read from an ingress-referenced secret
type cachedTLS struct {
	cert       string
	privateKey string
}

// getRouteTLS converts cached tls to route tls
func (ctls *cachedTLS) getRouteTLS() *routeapi.TLSConfig {
	// Defaults will be used for route tls options that cannot
	// be configured from an ingress resource:
	//
	//  - Termination
	//  - CACertificate
	//  - DestinationCACertificate
	//  - InsecureEdgeTerminationPolicy
	//
	// TODO Until ingress is extended to support the
	// necessary configuration (directly or via
	// annotations), it may be desirable to set defaults
	// via router options.
	return &routeapi.TLSConfig{
		// From SetDefaults_TLSConfig
		Termination: routeapi.TLSTerminationEdge,

		InsecureEdgeTerminationPolicy: routeapi.InsecureEdgeTerminationPolicyAllow,

		Certificate: ctls.cert,
		Key:         ctls.privateKey,
	}
}

// NewIngressTranslator creates a new cache for the given client
func NewIngressTranslator(kc kcoreclient.SecretsGetter) *IngressTranslator {
	return &IngressTranslator{
		ingressMap: make(map[string]*extensions.Ingress),
		tlsMap:     make(map[string]*referencedTLS),
		client:     kc,
	}
}

// TranslateIngressEvent converts an ingress event into route events.
func (it *IngressTranslator) TranslateIngressEvent(eventType watch.EventType, ingress *extensions.Ingress) []ingressRouteEvents {
	it.lock.Lock()
	defer it.lock.Unlock()

	// Do not return events for ingresses outside the set of allowed namespaces
	if it.allowedNamespaces != nil && !it.allowedNamespaces.Has(ingress.Namespace) {
		return []ingressRouteEvents{}
	}

	events := it.unsafeTranslateIngressEvent(eventType, ingress)
	return []ingressRouteEvents{events}
}

// TranslateSecretEvent converts the given secret event into route events.
func (it *IngressTranslator) TranslateSecretEvent(eventType watch.EventType, secret *kapi.Secret) (events []ingressRouteEvents) {
	key := getResourceKey(secret.ObjectMeta)

	events = []ingressRouteEvents{}

	it.lock.Lock()
	defer it.lock.Unlock()

	refTLS := it.tlsMap[key]
	if refTLS == nil {
		// If the secret has not been cached, no referencing ingresses
		// have been seen and the event can be ignored.
		return
	}

	switch eventType {
	case watch.Added, watch.Modified:
		newTLS, err := tlsFromSecret(secret)
		// If an error was encountered, newTLS will be nil and tls
		// previously cached for the secret will be removed.
		if err != nil {
			glog.V(4).Info(err)
		}

		// Avoid returning events for tls that hasn't changed
		if newTLS == refTLS.tls {
			return
		}

		refTLS.tls = newTLS
	case watch.Deleted:
		// Clear the tls but don't remove the cache entry.  It's
		// desirable to keep track of secrets referenced by
		// ingresses even where no tls data exists to ensure that
		// the tls cache can be updated by future secret events and
		// that secret retrieval is only attempted the first time it
		// is referenced rather than for every ingress.
		refTLS.tls = nil
	}

	// Generate route events for ingresses that reference this secret
	for key := range refTLS.ingressKeys {
		ingress := it.ingressMap[key]
		routeEvents := it.unsafeTranslateIngressEvent(watch.Modified, ingress)
		events = append(events, routeEvents)
	}

	return
}

// UpdateNamespaces sets which namespaces ingress objects are allowed from and updates the cache accordingly.
func (it *IngressTranslator) UpdateNamespaces(namespaces sets.String) {
	it.lock.Lock()
	defer it.lock.Unlock()

	it.allowedNamespaces = namespaces

	for _, ingress := range it.ingressMap {
		if !namespaces.Has(ingress.Namespace) {
			it.dereferenceIngress(ingress)
		}
	}
}

// unsafeTranslateIngressEvent converts an ingress event into route events without
// applying a lock so that it can be called by both of the Translate methods.
func (it *IngressTranslator) unsafeTranslateIngressEvent(eventType watch.EventType, ingress *extensions.Ingress) ingressRouteEvents {
	ingressKey := getResourceKey(ingress.ObjectMeta)

	// Get a reference to an existing ingress before updating the cache
	oldIngress := it.ingressMap[ingressKey]

	// Update cache state
	it.handleIngressEvent(eventType, ingress, oldIngress)

	routeEvents := it.generateRouteEvents(eventType, ingress, oldIngress)

	return ingressRouteEvents{
		ingressKey:  ingressKey,
		routeEvents: routeEvents,
	}
}

// handleIngressEvent updates the cache of the translator in response to the provided ingress event.
func (it *IngressTranslator) handleIngressEvent(eventType watch.EventType, ingress, oldIngress *extensions.Ingress) {
	switch eventType {
	case watch.Added, watch.Modified:
		key := getResourceKey(ingress.ObjectMeta)
		it.ingressMap[key] = ingress
		it.cacheTLS(ingress.Spec.TLS, ingress.Namespace, key)
	case watch.Deleted:
		it.dereferenceIngress(oldIngress)
	}
}

// generateRouteEvents computes route events implied by an ingress event.  The old ingress is used in computing deletions.
func (it *IngressTranslator) generateRouteEvents(eventType watch.EventType, ingress, oldIngress *extensions.Ingress) []routeEvent {
	routeEvents := []routeEvent{}

	// Compute the routes for the ingress
	var routeNames sets.String
	var routes []*routeapi.Route
	if eventType == watch.Deleted {
		// A deleted ingress implies no routes
	} else {
		// Process ingress into routes even if tls configuration was not cached.  So long as the default insecure
		// edge termination policy is 'allow', there's no point in refusing to serve routes for which tls
		// configuration may be missing.
		//
		// TODO Revisit this if/when edge termination policy becomes configurable.
		routes, routeNames = ingressToRoutes(ingress)
	}

	if oldIngress != nil {
		// Diff the routes from an existing ingress and the new ingress to ensure:
		//
		// - rule deletion results in a deletion event for a route that was
		//   generated from the rule
		//
		// - a deleted ingress (indicated by an empty route map) results in deletion
		//   events for all routes generated from the previous ingress.
		oldRoutes, _ := ingressToRoutes(oldIngress)
		for _, oldRoute := range oldRoutes {
			if !routeNames.Has(oldRoute.Name) {
				// Not necessary to add TLS to routes marked for deletion.
				routeEvents = append(routeEvents, routeEvent{
					eventType: watch.Deleted,
					route:     oldRoute,
				})
			}
		}
	}

	// Add events for all routes in the route map.  Assume tls has been cached.
	for _, route := range routes {
		tls := it.tlsForHost(ingress.Spec.TLS, ingress.Namespace, route.Spec.Host)
		if tls != nil {
			route.Spec.TLS = tls.getRouteTLS()
		}
		routeEvents = append(routeEvents, routeEvent{
			eventType: eventType,
			route:     route,
		})
	}

	return routeEvents
}

// cacheTLS attempts to populate the tls cache for all secrets referenced by an ingress.
func (it *IngressTranslator) cacheTLS(ingressTLS []extensions.IngressTLS, namespace, ingressKey string) bool {
	success := true

	for _, tls := range ingressTLS {
		secretKey := getKey(namespace, tls.SecretName)

		var refTLS *referencedTLS

		if refTLS = it.tlsMap[secretKey]; refTLS == nil {
			// Attempt to retrieve a secret the first time it is referenced

			refTLS = &referencedTLS{
				ingressKeys: sets.String{},
			}

			it.tlsMap[secretKey] = refTLS

			// Attempt to retrieve the secret and read tls configuration from it.
			// If retrieval or reading fails, the tls for the cache entry will be
			// nil and only be populated in response to a subsequent event for the
			// secret that provides valid tls data.
			//
			// TODO should initial retrieval be retried to minimize the chances
			// that a temporary connectivity failure delays route availability
			// until the next sync event (when the secret will next be seen)?
			if secret, err := it.client.Secrets(namespace).Get(tls.SecretName, metav1.GetOptions{}); err != nil {
				glog.V(4).Infof("Error retrieving secret %v: %v", secretKey, err)
			} else {
				newTLS, err := tlsFromSecret(secret)
				// If an error was encountered, newTLS will be nil
				if err != nil {
					glog.V(4).Info(err)
				}
				refTLS.tls = newTLS
			}
		}

		// Indicate that the secret is referenced by this ingress
		refTLS.ingressKeys.Insert(ingressKey)

		if refTLS.tls == nil {
			glog.V(4).Infof("Unable to source TLS configuration for ingress %v from secret %v", ingressKey, secretKey)
			success = false
		}
	}

	return success
}

// dereferenceIngress ensures the given ingress is removed from the cache.
func (it *IngressTranslator) dereferenceIngress(ingress *extensions.Ingress) {
	if ingress != nil {
		key := getResourceKey(ingress.ObjectMeta)
		delete(it.ingressMap, key)
		it.dereferenceTLS(ingress.Spec.TLS, ingress.Namespace, key)
	}
}

// dereferenceTLS removes references from ingress tls to cached tls.
func (it *IngressTranslator) dereferenceTLS(ingressTLS []extensions.IngressTLS, namespace, ingressKey string) {
	for _, tls := range ingressTLS {
		secretKey := getKey(namespace, tls.SecretName)
		if refTLS, ok := it.tlsMap[secretKey]; ok {
			refTLS.ingressKeys.Delete(ingressKey)

			// TLS that is no longer referenced can be deleted
			if len(refTLS.ingressKeys) == 0 {
				delete(it.tlsMap, secretKey)
			}
		}
	}
}

// tlsForHost attempts to retrieve ingress tls configuration for the given host
func (it *IngressTranslator) tlsForHost(ingressTLS []extensions.IngressTLS, namespace, host string) (match *cachedTLS) {
	for _, tls := range ingressTLS {
		for _, tlsHost := range tls.Hosts {
			if len(host) == 0 {
				// Only match an empty host to tls without hosts defined
				if len(tlsHost) > 0 {
					continue
				}
			} else if !matchesHost(tlsHost, host) {
				continue
			}

			key := getKey(namespace, tls.SecretName)
			refTLS := it.tlsMap[key]
			if refTLS == nil {
				// A cache entry should exist for each secret referenced by an ingress,
				// so this condition indicates a serious problem.
				glog.Errorf("Secret %v missing from the ingress translator cache", key)
				continue
			}

			// Pick the first tls that matches the host.  Continue iterating over all tls
			// to allow logging of subsequent matches.
			if match == nil {
				match = refTLS.tls
			} else {
				if len(host) == 0 {
					host = "<no value>"
				}
				glog.Warningf("More than one tls configuration matches host: %s", host)
			}
		}
	}
	return
}

// tlsFromSecret attempts to read tls config from a secret.  If an error is
// encountered, nil config will be returned.
func tlsFromSecret(secret *kapi.Secret) (*cachedTLS, error) {
	// TODO validate that the cert and key are within reasonable size limits

	var cert, privateKey string
	msgs := []string{}

	// Read the cert
	rawCert, ok := secret.Data[kapi.TLSCertKey]
	if ok && len(rawCert) > 0 {
		cert = string(rawCert)
	} else {
		msgs = append(msgs, "invalid cert")
	}

	// Read the key
	rawPrivateKey, ok := secret.Data[kapi.TLSPrivateKeyKey]
	if ok && len(rawPrivateKey) > 0 {
		privateKey = string(rawPrivateKey)
	} else {
		msgs = append(msgs, "invalid private key")
	}

	// Return tls only if both fields were loaded without error
	if len(msgs) == 0 {
		return &cachedTLS{
			cert:       cert,
			privateKey: privateKey,
		}, nil
	} else {
		secretKey := getResourceKey(secret.ObjectMeta)
		return nil, fmt.Errorf("Unable to read TLS configuration from secret %v: %v", secretKey, strings.Join(msgs, " and "))
	}
}

// ingressToRoutes generates routes implied by the provided ingress.
//
// TLS configuration is intended to be applied separately to avoid
// coupling route generation to cache state.
func ingressToRoutes(ingress *extensions.Ingress) (routes []*routeapi.Route, routeNames sets.String) {
	routeNames = sets.String{}
	routes = []*routeapi.Route{}

	if ingress.Spec.Backend != nil {
		key := getResourceKey(ingress.ObjectMeta)
		glog.V(4).Infof("The default backend defined for ingress %v will be ignored.  Default backends are not compatible with the OpenShift router.", key)
	}

	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			name := generateRouteName(ingress.Name, rule.Host, path.Path)
			if routeNames.Has(name) {
				glog.V(4).Infof("Ingress %s has more than one rule for host '%s' and path '%s'",
					ingress.Name, rule.Host, path.Path)
				continue
			}

			route := &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					// Reuse the values from ingress
					Namespace:         ingress.Namespace,
					CreationTimestamp: ingress.CreationTimestamp,
					// Append a hash of the name to the ingress uid to ensure both uniqueness
					// and consistent sorting.
					UID: types.UID(fmt.Sprintf("%s-%x", ingress.UID, md5.Sum([]byte(name)))),
				},
				Spec: routeapi.RouteSpec{
					Host: rule.Host,
					Path: path.Path,
					To: routeapi.RouteTargetReference{
						Name: path.Backend.ServiceName,
					},
					Port: &routeapi.RoutePort{
						TargetPort: path.Backend.ServicePort,
					},
				},
			}

			// The router depends on defaults being set on resource creation.  Manually
			// set the same defaults for a generated route to ensure compatibility.
			//
			// TODO Consider round tripping the route through the api
			// conversion to simplify maintenance of defaults.

			// From SetDefaults_RouteSpec
			route.Spec.WildcardPolicy = routeapi.WildcardPolicyNone

			// From SetDefaults_RouteTargetReference
			route.Spec.To.Kind = "Service"
			route.Spec.To.Weight = new(int32)
			*route.Spec.To.Weight = 100

			routeNames.Insert(name)
			routes = append(routes, route)
		}
	}
	return
}

// matchHost checks whether a pattern (which can represent a wildcard
// domain of the form *.[subdomain]) matches the given host.
func matchesHost(pattern, host string) bool {
	if len(pattern) == 0 || len(host) == 0 {
		return false
	}

	patternParts := strings.Split(pattern, ".")
	hostParts := strings.Split(host, ".")

	if len(patternParts) != len(hostParts) {
		return false
	}

	for i, patternPart := range patternParts {
		if i == 0 && patternPart == "*" {
			continue
		}
		if patternPart != hostParts[i] {
			return false
		}
	}
	return true
}

func getKey(namespace, name string) string {
	return fmt.Sprintf("%v/%v", namespace, name)
}

// generateRouteName returns a stable route name for an ingress name,
// host and path.  It's fine if the host or path happen to be empty.
func generateRouteName(name, host, path string) string {
	// Routes generated from ingress rules contain '/' to prevent name
	// clashes with user-defined routes.  '/' is not permitted in a resource
	// name submitted to the api, but generated routes are only intended to
	// be used in the context of the router controller.
	//
	// Hash the path to ensure compatibility with inclusion in haproxy configuration
	return fmt.Sprintf("ingress/%s/%s/%x", name, host, md5.Sum([]byte(path)))
}

// IsGeneratedRoute indicates whether the given route name was generated from an ingress.
func IsGeneratedRouteName(name string) bool {
	// The name of a route generated from an ingress rule contains '/'
	// to prevent name clashes with user-defined routes.  See generateRouteName
	return strings.Index(name, "/") != -1
}

// GetNameForHost returns the name of the ingress if the route name was
// generated from a path, otherwise it returns the name as given.
func GetNameForHost(name string) string {
	if IsGeneratedRouteName(name) {
		// Use the ingress name embedded in the route name for the purposes
		// of generating a host.  The name of routes generated from ingress
		// rules will be 'ingress/[name]/[host]/[path]' (see generateRouteName).
		nameParts := strings.Split(name, "/")
		return nameParts[1]
	}
	return name
}

// GetSafeRouteName returns a name that is safe for use in an HAproxy config.
func GetSafeRouteName(name string) string {
	if IsGeneratedRouteName(name) {
		// The name of a route generated from an ingress path will contain '/', which
		// isn't compatible with HAproxy or F5.
		return strings.Replace(name, "/", ":", -1)
	}
	return name
}

// TODO this should probably be removed and replaced with an upstream keyfunc
// getResourceKey returns a string of the form [namespace]/[name] for
// the given resource.  This is a common way of ensuring a key for a
// resource that is unique across the cluster.
func getResourceKey(obj metav1.ObjectMeta) string {
	return fmt.Sprintf("%s/%s", obj.Namespace, obj.Name)
}
