package controller

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/origin/pkg/router"
	"github.com/openshift/origin/pkg/router/controller/hostindex"
)

// RouteHostFunc returns a host for a route. It may return an empty string.
type RouteHostFunc func(*routev1.Route) string

// HostForRoute returns the host set on the route.
func HostForRoute(route *routev1.Route) string {
	return route.Spec.Host
}

// UniqueHost implements the router.Plugin interface to provide
// a template based, backend-agnostic router.
type UniqueHost struct {
	plugin router.Plugin

	recorder RejectionRecorder

	// nil means different than empty
	allowedNamespaces sets.String

	// index tracks the set of active routes and the set of routes
	// that cannot be admitted due to ownership restrictions
	index hostindex.Interface
}

// NewUniqueHost creates a plugin wrapper that ensures only unique routes are passed into
// the underlying plugin. Recorder is an interface for indicating why a route was
// rejected.
func NewUniqueHost(plugin router.Plugin, disableOwnershipCheck bool, recorder RejectionRecorder) *UniqueHost {
	routeActivationFn := hostindex.SameNamespace
	if disableOwnershipCheck {
		routeActivationFn = hostindex.OldestFirst
	}
	return &UniqueHost{
		plugin: plugin,

		recorder: recorder,

		index: hostindex.New(routeActivationFn),
	}
}

// RoutesForHost is a helper that allows routes to be retrieved.
func (p *UniqueHost) RoutesForHost(host string) ([]*routev1.Route, bool) {
	routes, ok := p.index.RoutesForHost(host)
	return routes, ok
}

// HostLen returns the number of hosts currently tracked by this plugin.
func (p *UniqueHost) HostLen() int {
	return p.index.HostLen()
}

// HandleEndpoints processes watch events on the Endpoints resource.
func (p *UniqueHost) HandleEndpoints(eventType watch.EventType, endpoints *kapi.Endpoints) error {
	if p.allowedNamespaces != nil && !p.allowedNamespaces.Has(endpoints.Namespace) {
		return nil
	}
	return p.plugin.HandleEndpoints(eventType, endpoints)
}

// HandleNode processes watch events on the Node resource and calls the router
func (p *UniqueHost) HandleNode(eventType watch.EventType, node *kapi.Node) error {
	return p.plugin.HandleNode(eventType, node)
}

// HandleRoute processes watch events on the Route resource.
// TODO: this function can probably be collapsed with the router itself, as a function that
//   determines which component needs to be recalculated (which template) and then does so
//   on demand.
func (p *UniqueHost) HandleRoute(eventType watch.EventType, route *routev1.Route) error {
	if p.allowedNamespaces != nil && !p.allowedNamespaces.Has(route.Namespace) {
		return nil
	}

	routeName := routeNameKey(route)
	host := route.Spec.Host

	if len(host) == 0 {
		glog.V(4).Infof("Route %s has no host value", routeName)
		p.recorder.RecordRouteRejection(route, "NoHostValue", "no host value was defined for the route")
		p.plugin.HandleRoute(watch.Deleted, route)
		return nil
	}

	// Validate that the route host name conforms to DNS requirements.
	// Defends against routes created before validation rules were added for host names.
	if errs := ValidateHostName(route); len(errs) > 0 {
		glog.V(4).Infof("Route %s - invalid host name %s", routeName, host)
		errMessages := make([]string, len(errs))
		for i := 0; i < len(errs); i++ {
			errMessages[i] = errs[i].Error()
		}

		err := fmt.Errorf("host name validation errors: %s", strings.Join(errMessages, ", "))
		p.recorder.RecordRouteRejection(route, "InvalidHost", err.Error())
		p.plugin.HandleRoute(watch.Deleted, route)
		return err
	}

	// Add the route to the index and see whether it is exposed. If this change results in
	// other routes being exposed, notify the lower plugin. Report back to the end user when
	// their route does not get exposed.
	switch eventType {
	case watch.Deleted:
		glog.V(4).Infof("Deleting route %s", routeName)

		changes := p.index.Remove(route)
		owner := "<unknown>"
		if old, ok := p.index.RoutesForHost(host); ok && len(old) > 0 {
			owner = old[0].Namespace
		}

		// perform activations first so that the other routes exist before we alter this route
		for _, other := range changes.GetActivated() {
			if err := p.plugin.HandleRoute(watch.Added, other); err != nil {
				utilruntime.HandleError(fmt.Errorf("unable to activate route %s/%s that was previously hidden by another route: %v", other.Namespace, other.Name, err))
			}
		}

		// displaced routes must be deleted in nested plugins
		for _, other := range changes.GetDisplaced() {
			glog.V(4).Infof("route %s being deleted caused %s/%s to no longer be exposed", routeName, other.Namespace, other.Name)
			p.recorder.RecordRouteRejection(other, "HostAlreadyClaimed", fmt.Sprintf("namespace %s owns hostname %s", owner, host))

			if err := p.plugin.HandleRoute(watch.Deleted, other); err != nil {
				utilruntime.HandleError(fmt.Errorf("unable to clear route %s/%s that was previously exposed: %v", other.Namespace, other.Name, err))
			}
		}

		return p.plugin.HandleRoute(eventType, route)

	case watch.Added, watch.Modified:
		removed := false

		var nestedErr error
		changes, newRoute := p.index.Add(route)

		// perform activations first so that the other routes exist before we alter this route
		for _, other := range changes.GetActivated() {
			// we activated other routes
			if other != route {
				if err := p.plugin.HandleRoute(watch.Added, other); err != nil {
					utilruntime.HandleError(fmt.Errorf("unable to activate route %s/%s that was previously hidden by another route: %v", other.Namespace, other.Name, err))
				}
				continue
			}
			nestedErr = p.plugin.HandleRoute(eventType, other)
		}

		// displaced routes must be deleted in nested plugins
		for _, other := range changes.GetDisplaced() {
			// adding this route displaced others
			if other != route {
				glog.V(4).Infof("route %s will replace path %s from %s because it is older", routeName, route.Spec.Path, other.Name)
				p.recorder.RecordRouteRejection(other, "HostAlreadyClaimed", fmt.Sprintf("replaced by older route %s", route.Name))

				if err := p.plugin.HandleRoute(watch.Deleted, other); err != nil {
					utilruntime.HandleError(fmt.Errorf("unable to clear route %s/%s that was previously exposed: %v", other.Namespace, other.Name, err))
				}
				continue
			}

			// we were not added because another route is covering us
			removed = true
			var owner *routev1.Route
			if old, ok := p.index.RoutesForHost(host); ok && len(old) > 0 {
				owner = old[0]
			} else {
				owner = &routev1.Route{}
				owner.Name = "<unknown>"
			}
			glog.V(4).Infof("Route %s cannot take %s from %s/%s", routeName, host, owner.Namespace, owner.Name)
			if owner.Namespace == route.Namespace {
				p.recorder.RecordRouteRejection(route, "HostAlreadyClaimed", fmt.Sprintf("route %s already exposes %s and is older", owner.Name, host))
			} else {
				p.recorder.RecordRouteRejection(route, "HostAlreadyClaimed", fmt.Sprintf("a route in another namespace holds %s and is older than %s", host, route.Name))
			}

			// if this is the first time we've seen this route, we don't have to notify nested plugins
			if !newRoute {
				// indicate to lower plugins that the route should not be shown
				if err := p.plugin.HandleRoute(watch.Deleted, route); err != nil {
					utilruntime.HandleError(fmt.Errorf("unable to clear route %s: %v", routeName, err))
				}
			}
		}

		// // ensure we pass down modifications
		// if !added && !removed {
		// 	if err := p.plugin.HandleRoute(watch.Modified, route); err != nil {
		// 		utilruntime.HandleError(fmt.Errorf("unable to modify route %s: %v", routeName, err))
		// 	}
		// }

		if removed {
			return fmt.Errorf("another route has claimed this host")
		}
		return nestedErr

	default:
		return fmt.Errorf("unrecognized watch type: %v", eventType)
	}
}

// HandleNamespaces limits the scope of valid routes to only those that match
// the provided namespace list.
func (p *UniqueHost) HandleNamespaces(namespaces sets.String) error {
	p.allowedNamespaces = namespaces
	p.index.Filter(func(route *routev1.Route) bool {
		return namespaces.Has(route.Namespace)
	})
	return p.plugin.HandleNamespaces(namespaces)
}

// Commit invokes the nested plugin to commit.
func (p *UniqueHost) Commit() error {
	return p.plugin.Commit()
}

// routeNameKey returns a unique name for a given route
func routeNameKey(route *routev1.Route) string {
	return fmt.Sprintf("%s/%s", route.Namespace, route.Name)
}

// ValidateHostName checks that a route's host name satisfies DNS requirements.
func ValidateHostName(route *routev1.Route) field.ErrorList {
	result := field.ErrorList{}
	if len(route.Spec.Host) < 1 {
		return result
	}

	specPath := field.NewPath("spec")
	hostPath := specPath.Child("host")

	if len(kvalidation.IsDNS1123Subdomain(route.Spec.Host)) != 0 {
		result = append(result, field.Invalid(hostPath, route.Spec.Host, "host must conform to DNS 952 subdomain conventions"))
	}

	segments := strings.Split(route.Spec.Host, ".")
	for _, s := range segments {
		errs := kvalidation.IsDNS1123Label(s)
		for _, e := range errs {
			result = append(result, field.Invalid(hostPath, route.Spec.Host, e))
		}
	}

	return result
}
