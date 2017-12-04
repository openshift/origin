package controller

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	"github.com/openshift/origin/pkg/route/apis/route/validation"
	"github.com/openshift/origin/pkg/router"
)

// ExtendedValidator implements the router.Plugin interface to provide
// extended config validation for template based, backend-agnostic routers.
type ExtendedValidator struct {
	// plugin is the next plugin in the chain.
	plugin router.Plugin

	// recorder is an interface for indicating route rejections.
	recorder RejectionRecorder

	// invalidRoutes is a map of invalid routes previously encountered.
	invalidRoutes map[string]routeapi.Route
}

// NewExtendedValidator creates a plugin wrapper that ensures only routes that
// pass extended validation are relayed to the next plugin in the chain.
// Recorder is an interface for indicating why a route was rejected.
func NewExtendedValidator(plugin router.Plugin, recorder RejectionRecorder) *ExtendedValidator {
	return &ExtendedValidator{
		plugin:        plugin,
		recorder:      recorder,
		invalidRoutes: make(map[string]routeapi.Route),
	}
}

// HandleNode processes watch events on the node resource
func (p *ExtendedValidator) HandleNode(eventType watch.EventType, node *kapi.Node) error {
	return p.plugin.HandleNode(eventType, node)
}

// HandleEndpoints processes watch events on the Endpoints resource.
func (p *ExtendedValidator) HandleEndpoints(eventType watch.EventType, endpoints *kapi.Endpoints) error {
	return p.plugin.HandleEndpoints(eventType, endpoints)
}

// HandleRoute processes watch events on the Route resource.
func (p *ExtendedValidator) HandleRoute(eventType watch.EventType, route *routeapi.Route) error {
	// Check if previously seen route and its Spec is unchanged.
	routeName := routeNameKey(route)
	old, ok := p.invalidRoutes[routeName]
	if ok && reflect.DeepEqual(old.Spec, route.Spec) {
		// Route spec was unchanged and it is already marked in
		// error, we don't need to do anything more.
		return fmt.Errorf("invalid route configuration")
	}

	if errs := validation.ExtendedValidateRoute(route); len(errs) > 0 {
		errmsg := ""
		for i := 0; i < len(errs); i++ {
			errmsg = errmsg + "\n  - " + errs[i].Error()
		}
		glog.Errorf("Skipping route %s due to invalid configuration: %s", routeName, errmsg)

		p.recorder.RecordRouteRejection(route, "ExtendedValidationFailed", errmsg)
		return fmt.Errorf("invalid route configuration")
	}

	return p.plugin.HandleRoute(eventType, route)
}

// HandleNamespaces limits the scope of valid routes to only those that match
// the provided namespace list.
func (p *ExtendedValidator) HandleNamespaces(namespaces sets.String) error {
	return p.plugin.HandleNamespaces(namespaces)
}

func (p *ExtendedValidator) Commit() error {
	return p.plugin.Commit()
}
