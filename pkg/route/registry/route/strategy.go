package route

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/route"
	"github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/route/api/validation"
)

// HostGeneratedAnnotationKey is the key for an annotation set to "true" if the route's host was generated
const HostGeneratedAnnotationKey = "openshift.io/host.generated"

type routeStrategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
	route.RouteAllocator
}

// NewStrategy initializes the default logic that applies when creating and updating
// Route objects via the REST API.
func NewStrategy(allocator route.RouteAllocator) routeStrategy {
	return routeStrategy{
		kapi.Scheme,
		kapi.SimpleNameGenerator,
		allocator,
	}
}

func (routeStrategy) NamespaceScoped() bool {
	return true
}

func (s routeStrategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
	route := obj.(*api.Route)
	route.Status = api.RouteStatus{}
	err := s.allocateHost(route)
	if err != nil {
		// TODO: this will be changed when moved to a controller
		utilruntime.HandleError(errors.NewInternalError(fmt.Errorf("allocation error: %v for route: %#v", err, obj)))
	}
}

func (s routeStrategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	route := obj.(*api.Route)
	oldRoute := old.(*api.Route)
	route.Status = oldRoute.Status

	// Ignore attempts to clear the spec Host
	// Prevents "immutable field" errors when applying the same route definition used to create
	if len(route.Spec.Host) == 0 {
		route.Spec.Host = oldRoute.Spec.Host
	}
}

// allocateHost allocates a host name ONLY if the route doesn't specify a subdomain wildcard policy and
// the host name on the route is empty and an allocator is configured.
// It must first allocate the shard and may return an error if shard allocation fails.
func (s routeStrategy) allocateHost(route *api.Route) error {
	if route.Spec.WildcardPolicy == api.WildcardPolicySubdomain {
		// Don't allocate a host if subdomain wildcard policy.
		return nil
	}

	if len(route.Spec.Host) == 0 && s.RouteAllocator != nil {
		// TODO: this does not belong here, and should be removed
		shard, err := s.RouteAllocator.AllocateRouterShard(route)
		if err != nil {
			return errors.NewInternalError(fmt.Errorf("allocation error: %v for route: %#v", err, route))
		}
		route.Spec.Host = s.RouteAllocator.GenerateHostname(route, shard)
		if route.Annotations == nil {
			route.Annotations = map[string]string{}
		}
		route.Annotations[HostGeneratedAnnotationKey] = "true"
	}
	return nil
}

func (routeStrategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	route := obj.(*api.Route)
	return validation.ValidateRoute(route)
}

func (routeStrategy) AllowCreateOnUpdate() bool {
	return false
}

// Canonicalize normalizes the object after validation.
func (routeStrategy) Canonicalize(obj runtime.Object) {
}

func (routeStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	oldRoute := old.(*api.Route)
	objRoute := obj.(*api.Route)
	return validation.ValidateRouteUpdate(objRoute, oldRoute)
}

func (routeStrategy) AllowUnconditionalUpdate() bool {
	return true
}

type routeStatusStrategy struct {
	routeStrategy
}

var StatusStrategy = routeStatusStrategy{NewStrategy(nil)}

func (routeStatusStrategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
	newRoute := obj.(*api.Route)
	oldRoute := old.(*api.Route)
	newRoute.Spec = oldRoute.Spec
}

func (routeStatusStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateRouteStatusUpdate(obj.(*api.Route), old.(*api.Route))
}

// Matcher returns a matcher for a route
func Matcher(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
	return &generic.SelectionPredicate{Label: label, Field: field, GetAttrs: getAttrs}
}

func getAttrs(obj runtime.Object) (objLabels labels.Set, objFields fields.Set, err error) {
	route := obj.(*api.Route)
	return labels.Set(route.Labels), api.RouteToSelectableFields(route), nil
}
