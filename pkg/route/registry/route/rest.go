package route

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/fielderrors"

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

func (s routeStrategy) PrepareForCreate(obj runtime.Object) {
	route := obj.(*api.Route)
	route.Status = api.RouteStatus{}
	if len(route.Spec.Host) == 0 && s.RouteAllocator != nil {
		// TODO: this does not belong here, and should be removed
		shard, err := s.RouteAllocator.AllocateRouterShard(route)
		if err != nil {
			// TODO: this will be changed when moved to a controller
			util.HandleError(errors.NewInternalError(fmt.Errorf("allocation error: %v for route: %#v", err, obj)))
			return
		}
		route.Spec.Host = s.RouteAllocator.GenerateHostname(route, shard)
		if route.Annotations == nil {
			route.Annotations = map[string]string{}
		}
		route.Annotations[HostGeneratedAnnotationKey] = "true"
	}
}

func (routeStrategy) PrepareForUpdate(obj, old runtime.Object) {
	route := obj.(*api.Route)
	oldRoute := old.(*api.Route)
	route.Status = oldRoute.Status
}

func (routeStrategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	route := obj.(*api.Route)
	return validation.ValidateRoute(route)
}

func (routeStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (routeStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
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

func (routeStatusStrategy) PrepareForUpdate(obj, old runtime.Object) {
	newRoute := obj.(*api.Route)
	oldRoute := old.(*api.Route)
	newRoute.Spec = oldRoute.Spec
}

func (routeStatusStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateRouteStatusUpdate(obj.(*api.Route), old.(*api.Route))
}

// MatchRoute returns a matcher for a route
func MatchRoute(label labels.Selector, field fields.Selector) generic.Matcher {
	return &generic.SelectionPredicate{label, field, getAttrs}
}

func getAttrs(obj runtime.Object) (objLabels labels.Set, objFields fields.Set, err error) {
	route := obj.(*api.Route)
	return labels.Set(route.Labels), RouteToSelectableFields(route), nil
}

// RouteToSelectableFields returns a label set that represents the object
func RouteToSelectableFields(route *api.Route) fields.Set {
	return fields.Set{
		"metadata.name": route.Name,
		"spec.path":     route.Spec.Path,
		"spec.host":     route.Spec.Host,
		"spec.to.name":  route.Spec.To.Name,
	}
}
