package admission

import (
	"errors"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"fmt"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	osclient "github.com/openshift/origin/pkg/client"
	routeapi "github.com/openshift/origin/pkg/route/api"
)

// This plugin must be manually initialized, it requires an openshift client which is not
// available via the factory interface normally used in NewFromPlugins.

// NewRouteAdmissionPlugin creates a new route admission plugin to enforce uniqueness.
func NewRouteAdmissionPlugin(kClient kclient.Interface, osClient osclient.Interface) (admission.Interface, error) {
	if kClient == nil || osClient == nil {
		return nil, errors.New("kube client and openshift client are required")
	}

	store := cache.NewStore(StoreKeyFunc)
	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return osClient.Routes(api.NamespaceAll).List(labels.Everything(), fields.Everything())
			},
			WatchFunc: func(resourceVersion string) (watch.Interface, error) {
				return osClient.Routes(api.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
			},
		},
		&routeapi.Route{},
		store,
		0,
	)
	reflector.Run()

	return &routeUniqueness{
		Handler:    routeAdmissionHandler(),
		kClient:    kClient,
		osClient:   osClient,
		routeStore: store,
	}, nil
}

// routeAdmissionHandler is separated out so that tests can use it in the test plugin.
func routeAdmissionHandler() *admission.Handler {
	return admission.NewHandler(admission.Create, admission.Update)
}

// routeUniqueness implements the plugin that validates route uniqueness.
type routeUniqueness struct {
	*admission.Handler
	kClient    kclient.Interface
	osClient   osclient.Interface
	routeStore cache.Store
}

// Admit ensures that the route in the form of scheme://host/path does not already exist in the
// system.  Rules for admission:
// 1.  If you're adding a route it must have a unique scheme, host, path combination
// 2.  If you're updating a route you may not update it to a scheme, host, path combination that
//     already exists (used by checking the namespace and name of the route)
func (p *routeUniqueness) Admit(a admission.Attributes) error {
	if a.GetResource() != "routes" {
		return nil
	}

	route, ok := a.GetObject().(*routeapi.Route)
	if !ok {
		return admission.NewForbidden(a, fmt.Errorf("Resource was marked with kind Route but was unable to be converted"))
	}
	routeKey := MakeRouteKey(route)

	// see if we can find a route that matches the scheme, host, path combination
	foundObj, exists, err := p.routeStore.GetByKey(routeKey)
	if err != nil {
		return admission.NewForbidden(a, err)
	}
	// if no existing route is found this is ok to create
	if !exists {
		return nil
	}
	// if updating then the namespace and name must match in order to accept the update
	found, ok := foundObj.(*routeapi.Route)
	if !ok {
		return admission.NewForbidden(a, fmt.Errorf("Invalid object found in store, unable to convert to route %v", foundObj))
	}

	// if adding and the route already exists then deny
	if a.GetOperation() == admission.Create && exists {
		return admission.NewForbidden(a, fmt.Errorf("Route %s already exists.  If you are the owner of this domain please contact support.", routeKey))
	}

	if found.Namespace != route.Namespace || found.Name != route.Name {
		return admission.NewForbidden(a, fmt.Errorf("Route %s already exists.  If you are the owner of this domain please contact support.", routeKey))
	}

	// found a route, uids match, ok to update
	return nil
}

// StoreKeyFunc provides the store a key function that produces keys in the format of MakeRouteKey
// below.
func StoreKeyFunc(obj interface{}) (string, error) {
	route, ok := obj.(*routeapi.Route)
	if !ok {
		return "", fmt.Errorf("Unable to convert object to route: %v", obj)
	}
	return MakeRouteKey(route), nil
}

// MakeRouteKey produces keys in the format of {scheme}{host}{path}.
func MakeRouteKey(route *routeapi.Route) string {
	if route == nil {
		return ""
	}
	scheme := "http://"
	if route.TLS != nil && route.TLS.Termination != "" {
		scheme = "https://"
	}
	return scheme + route.Host + route.Path
}
