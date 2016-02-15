package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	lru "github.com/hashicorp/golang-lru"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client"
	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
)

// StatusAdmitter ensures routes added to the plugin have status set.
type StatusAdmitter struct {
	plugin     router.Plugin
	client     client.RoutesNamespacer
	routerName string

	contentionInterval time.Duration
	expected           *lru.Cache
}

// NewStatusAdmitter creates a plugin wrapper that ensures every accepted
// route has a status field set that matches this router. The admitter manages
// an LRU of recently seen conflicting updates to handle when two router processes
// with differing configurations are writing updates at the same time.
func NewStatusAdmitter(plugin router.Plugin, client client.RoutesNamespacer, name string) *StatusAdmitter {
	expected, _ := lru.New(1024)
	return &StatusAdmitter{
		plugin:     plugin,
		client:     client,
		routerName: name,

		contentionInterval: 1 * time.Minute,
		expected:           expected,
	}
}

// nowFn allows the package to be tested
var nowFn = unversioned.Now

// findOrCreateIngress loops through the router status ingress array looking for an entry
// that matches name. If there is no entry in the array, it creates one and appends it
// to the array. If there are multiple entries with that name, the first one is
// returned and later ones are removed. Changed is returned as true if any part of the
// array is altered.
func findOrCreateIngress(route *routeapi.Route, name string) (_ *routeapi.RouteIngress, changed bool) {
	position := -1
	updated := make([]routeapi.RouteIngress, 0, len(route.Status.Ingress))
	for i := range route.Status.Ingress {
		existing := &route.Status.Ingress[i]
		if existing.RouterName != name {
			updated = append(updated, *existing)
			continue
		}
		if position != -1 {
			changed = true
			continue
		}
		updated = append(updated, *existing)
		position = i
	}
	switch {
	case position == -1:
		position = len(route.Status.Ingress)
		route.Status.Ingress = append(route.Status.Ingress, routeapi.RouteIngress{
			RouterName: name,
			Host:       route.Spec.Host,
		})
		changed = true
	case changed:
		route.Status.Ingress = updated
	}
	ingress := &route.Status.Ingress[position]
	if ingress.Host != route.Spec.Host {
		ingress.Host = route.Spec.Host
		changed = true
	}
	return ingress, changed
}

// setIngressCondition records the condition on the ingress, returning true if the ingress was changed and
// false if no modification was made.
func setIngressCondition(ingress *routeapi.RouteIngress, condition routeapi.RouteIngressCondition) bool {
	for _, existing := range ingress.Conditions {
		//existing.LastTransitionTime = nil
		if existing == condition {
			return false
		}
	}
	now := nowFn()
	condition.LastTransitionTime = &now
	ingress.Conditions = []routeapi.RouteIngressCondition{condition}
	return true
}

func ingressConditionTouched(ingress *routeapi.RouteIngress) *unversioned.Time {
	var lastTouch *unversioned.Time
	for _, condition := range ingress.Conditions {
		if t := condition.LastTransitionTime; t != nil {
			switch {
			case lastTouch == nil, t.After(lastTouch.Time):
				lastTouch = t
			}
		}
	}
	return lastTouch
}

// recordIngressConditionFailure updates the matching ingress on the route (or adds a new one) with the specified
// condition, returning true if the object was modified.
func recordIngressConditionFailure(route *routeapi.Route, name string, condition routeapi.RouteIngressCondition) (*routeapi.RouteIngress, bool, *unversioned.Time) {
	for i := range route.Status.Ingress {
		existing := &route.Status.Ingress[i]
		if existing.RouterName != name {
			continue
		}
		lastTouch := ingressConditionTouched(existing)
		return existing, setIngressCondition(existing, condition), lastTouch
	}
	route.Status.Ingress = append(route.Status.Ingress, routeapi.RouteIngress{RouterName: name})
	ingress := &route.Status.Ingress[len(route.Status.Ingress)-1]
	setIngressCondition(ingress, condition)
	return ingress, true, nil
}

// hasIngressBeenTouched returns true if the route appears to have been touched since the last time
func (a *StatusAdmitter) hasIngressBeenTouched(route *routeapi.Route, lastTouch *unversioned.Time) bool {
	glog.V(4).Infof("has last touch %v for %s/%s", lastTouch, route.Namespace, route.Name)
	if lastTouch.IsZero() {
		return false
	}
	old, ok := a.expected.Get(route.UID)
	if !ok || old.(time.Time).Equal(lastTouch.Time) {
		return false
	}
	return true
}

// recordIngressTouch
func (a *StatusAdmitter) recordIngressTouch(route *routeapi.Route, touch *unversioned.Time, err error) {
	switch {
	case err == nil:
		if touch != nil {
			a.expected.Add(route.UID, touch.Time)
		}
	case errors.IsConflict(err):
		a.expected.Add(route.UID, time.Time{})
	}
}

// admitRoute returns true if the route has already been accepted to this router, or
// updates the route to contain an accepted condition. Returns an error if the route could
// not be admitted.
func (a *StatusAdmitter) admitRoute(oc client.RoutesNamespacer, route *routeapi.Route, name string) (bool, error) {
	ingress, updated := findOrCreateIngress(route, name)
	if !updated {
		for i := range ingress.Conditions {
			cond := &ingress.Conditions[i]
			if cond.Type == routeapi.RouteAdmitted && cond.Status == kapi.ConditionTrue {
				glog.V(4).Infof("admit: route already admitted")
				return true, nil
			}
		}
	}

	if a.hasIngressBeenTouched(route, ingressConditionTouched(ingress)) {
		glog.V(4).Infof("admit: observed a route update from someone else: route %s/%s has been updated to an inconsistent value, doing nothing", route.Namespace, route.Name)
		return true, nil
	}

	setIngressCondition(ingress, routeapi.RouteIngressCondition{
		Type:   routeapi.RouteAdmitted,
		Status: kapi.ConditionTrue,
	})
	glog.V(4).Infof("admit: admitting route by updating status: %s (%t): %s", route.Name, updated, route.Spec.Host)
	_, err := oc.Routes(route.Namespace).UpdateStatus(route)
	a.recordIngressTouch(route, ingress.Conditions[0].LastTransitionTime, err)
	return err == nil, err
}

// RecordRouteRejection attempts to update the route status with a reason for a route being rejected.
func (a *StatusAdmitter) RecordRouteRejection(route *routeapi.Route, reason, message string) {
	ingress, changed, lastTouch := recordIngressConditionFailure(route, a.routerName, routeapi.RouteIngressCondition{
		Type:    routeapi.RouteAdmitted,
		Status:  kapi.ConditionFalse,
		Reason:  reason,
		Message: message,
	})
	if !changed {
		glog.V(4).Infof("reject: no changes to route needed: %s/%s", route.Namespace, route.Name)
		return
	}

	if a.hasIngressBeenTouched(route, lastTouch) {
		glog.V(4).Infof("reject: observed a route update from someone else: route %s/%s has been updated to an inconsistent value, doing nothing", route.Namespace, route.Name)
		return
	}

	_, err := a.client.Routes(route.Namespace).UpdateStatus(route)
	a.recordIngressTouch(route, ingress.Conditions[0].LastTransitionTime, err)
	if err != nil {
		util.HandleError(fmt.Errorf("unable to write route rejection to the status: %v", err))
	}
}

// HandleRoute attempts to admit the provided route on watch add / modifications.
func (a *StatusAdmitter) HandleRoute(eventType watch.EventType, route *routeapi.Route) error {
	switch eventType {
	case watch.Added, watch.Modified:
		ok, err := a.admitRoute(a.client, route, a.routerName)
		if err != nil {
			return err
		}
		if !ok {
			glog.V(4).Infof("skipping route: %s", route.Name)
			return nil
		}
	}
	return a.plugin.HandleRoute(eventType, route)
}

func (a *StatusAdmitter) HandleEndpoints(eventType watch.EventType, route *kapi.Endpoints) error {
	return a.plugin.HandleEndpoints(eventType, route)
}

func (a *StatusAdmitter) HandleNamespaces(namespaces sets.String) error {
	return a.plugin.HandleNamespaces(namespaces)
}
