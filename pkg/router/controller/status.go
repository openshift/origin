package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	routev1 "github.com/openshift/api/route/v1"
	client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	routelisters "github.com/openshift/client-go/route/listers/route/v1"
	"github.com/openshift/origin/pkg/router"
	"github.com/openshift/origin/pkg/util/writerlease"
)

// RejectionRecorder is an object capable of recording why a route was rejected
type RejectionRecorder interface {
	RecordRouteRejection(route *routev1.Route, reason, message string)
}

// LogRejections writes rejection messages to the log.
var LogRejections = logRecorder{}

type logRecorder struct{}

func (logRecorder) RecordRouteRejection(route *routev1.Route, reason, message string) {
	glog.V(3).Infof("Rejected route %s in namespace %s: %s: %s", route.Name, route.Namespace, reason, message)
}

// StatusAdmitter ensures routes added to the plugin have status set.
type StatusAdmitter struct {
	plugin router.Plugin
	client client.RoutesGetter
	lister routelisters.RouteLister

	routerName              string
	routerCanonicalHostname string

	lease   writerlease.Lease
	tracker ContentionTracker
}

// NewStatusAdmitter creates a plugin wrapper that ensures every accepted
// route has a status field set that matches this router. The admitter manages
// an LRU of recently seen conflicting updates to handle when two router processes
// with differing configurations are writing updates at the same time.
func NewStatusAdmitter(plugin router.Plugin, client client.RoutesGetter, lister routelisters.RouteLister, name, hostName string, lease writerlease.Lease, tracker ContentionTracker) *StatusAdmitter {
	return &StatusAdmitter{
		plugin: plugin,
		client: client,
		lister: lister,

		routerName:              name,
		routerCanonicalHostname: hostName,

		tracker: tracker,
		lease:   lease,
	}
}

// Return a time truncated to the second to ensure that in-memory and
// serialized timestamps can be safely compared.
func getRfc3339Timestamp() metav1.Time {
	return metav1.Now().Rfc3339Copy()
}

// nowFn allows the package to be tested
var nowFn = getRfc3339Timestamp

// HandleRoute attempts to admit the provided route on watch add / modifications.
func (a *StatusAdmitter) HandleRoute(eventType watch.EventType, route *routev1.Route) error {
	switch eventType {
	case watch.Added, watch.Modified:
		performIngressConditionUpdate("admit", a.lease, a.tracker, a.client, a.lister, route, a.routerName, a.routerCanonicalHostname, routev1.RouteIngressCondition{
			Type:   routev1.RouteAdmitted,
			Status: corev1.ConditionTrue,
		})
	}
	return a.plugin.HandleRoute(eventType, route)
}

func (a *StatusAdmitter) HandleNode(eventType watch.EventType, node *kapi.Node) error {
	return a.plugin.HandleNode(eventType, node)
}

func (a *StatusAdmitter) HandleEndpoints(eventType watch.EventType, route *kapi.Endpoints) error {
	return a.plugin.HandleEndpoints(eventType, route)
}

func (a *StatusAdmitter) HandleNamespaces(namespaces sets.String) error {
	return a.plugin.HandleNamespaces(namespaces)
}

func (a *StatusAdmitter) Commit() error {
	return a.plugin.Commit()
}

// RecordRouteRejection attempts to update the route status with a reason for a route being rejected.
func (a *StatusAdmitter) RecordRouteRejection(route *routev1.Route, reason, message string) {
	performIngressConditionUpdate("reject", a.lease, a.tracker, a.client, a.lister, route, a.routerName, a.routerCanonicalHostname, routev1.RouteIngressCondition{
		Type:    routev1.RouteAdmitted,
		Status:  corev1.ConditionFalse,
		Reason:  reason,
		Message: message,
	})
}

// performIngressConditionUpdate updates the route to the appropriate status for the provided condition.
func performIngressConditionUpdate(action string, lease writerlease.Lease, tracker ContentionTracker, oc client.RoutesGetter, lister routelisters.RouteLister, route *routev1.Route, routerName, hostName string, condition routev1.RouteIngressCondition) {
	attempts := 3
	key := string(route.UID)
	routeNamespace, routeName := route.Namespace, route.Name

	lease.Try(key, func() (writerlease.WorkResult, bool) {
		route, err := lister.Routes(routeNamespace).Get(routeName)
		if err != nil {
			return writerlease.None, false
		}
		if string(route.UID) != key {
			glog.V(4).Infof("%s: skipped update due to route UID changing (likely delete and recreate): %s/%s", action, route.Namespace, route.Name)
			return writerlease.None, false
		}

		route = route.DeepCopy()
		changed, created, now, latest, original := recordIngressCondition(route, routerName, hostName, condition)
		if !changed {
			glog.V(4).Infof("%s: no changes to route needed: %s/%s", action, route.Namespace, route.Name)
			// if the most recent change was to our ingress status, consider the current lease extended
			if findMostRecentIngress(route) == routerName {
				lease.Extend(key)
			}
			return writerlease.None, false
		}

		// If the tracker determines that another process is attempting to update the ingress to an inconsistent
		// value, skip updating altogether and rely on the next resync to resolve conflicts. This prevents routers
		// with different configurations from endlessly updating the route status.
		if !created && tracker.IsChangeContended(key, now, original) {
			glog.V(4).Infof("%s: skipped update due to another process altering the route with a different ingress status value: %s %#v", action, key, original)
			return writerlease.Release, false
		}

		switch _, err := oc.Routes(route.Namespace).UpdateStatus(route); {
		case err == nil:
			glog.V(4).Infof("%s: updated status of %s/%s", action, route.Namespace, route.Name)
			tracker.Clear(key, latest)
			return writerlease.Extend, false
		case errors.IsForbidden(err):
			// if the router can't write status updates, allow the route to go through
			utilruntime.HandleError(fmt.Errorf("Unable to write router status - please ensure you reconcile your system policy or grant this router access to update route status: %v", err))
			tracker.Clear(key, latest)
			return writerlease.Extend, false
		case errors.IsNotFound(err):
			// route was deleted
			glog.V(4).Infof("%s: route %s/%s was deleted before we could update status", action, route.Namespace, route.Name)
			return writerlease.Release, false
		case errors.IsConflict(err):
			// just follow the normal process, and retry when we receive the update notification due to
			// the other entity updating the route.
			glog.V(4).Infof("%s: updating status of %s/%s failed due to write conflict", action, route.Namespace, route.Name)
			return writerlease.Release, true
		default:
			utilruntime.HandleError(fmt.Errorf("Unable to write router status for %s/%s: %v", route.Namespace, route.Name, err))
			attempts--
			return writerlease.Release, attempts > 0
		}
	})
}

// recordIngressCondition updates the matching ingress on the route (or adds a new one) with the specified
// condition, returning whether the route was updated or created, the time assigned to the condition, and
// a pointer to the current ingress record.
func recordIngressCondition(route *routev1.Route, name, hostName string, condition routev1.RouteIngressCondition) (changed, created bool, at time.Time, latest, original *routev1.RouteIngress) {
	for i := range route.Status.Ingress {
		existing := &route.Status.Ingress[i]
		if existing.RouterName != name {
			continue
		}

		// check whether the ingress is out of date without modifying it
		changed := existing.Host != route.Spec.Host ||
			existing.WildcardPolicy != route.Spec.WildcardPolicy ||
			existing.RouterCanonicalHostname != hostName

		existingCondition := findCondition(existing, condition.Type)
		if existingCondition != nil {
			condition.LastTransitionTime = existingCondition.LastTransitionTime
			if *existingCondition != condition {
				changed = true
			}
		}
		if !changed {
			return false, false, time.Time{}, existing, existing
		}

		// preserve a copy of the original ingress without conditions
		original := *existing
		original.Conditions = nil

		// generate the correct ingress
		existing.Host = route.Spec.Host
		existing.WildcardPolicy = route.Spec.WildcardPolicy
		existing.RouterCanonicalHostname = hostName
		if existingCondition == nil {
			existing.Conditions = append(existing.Conditions, condition)
			existingCondition = &existing.Conditions[len(existing.Conditions)-1]
		} else {
			*existingCondition = condition
		}
		now := nowFn()
		existingCondition.LastTransitionTime = &now

		return true, false, now.Time, existing, &original
	}

	// add a new ingress
	route.Status.Ingress = append(route.Status.Ingress, routev1.RouteIngress{
		RouterName:              name,
		Host:                    route.Spec.Host,
		WildcardPolicy:          route.Spec.WildcardPolicy,
		RouterCanonicalHostname: hostName,
		Conditions: []routev1.RouteIngressCondition{
			condition,
		},
	})
	ingress := &route.Status.Ingress[len(route.Status.Ingress)-1]
	now := nowFn()
	ingress.Conditions[0].LastTransitionTime = &now

	return true, true, now.Time, ingress, nil
}

// findMostRecentIngress returns the name of the ingress status with the most recent Admitted condition transition time,
// or an empty string if no such ingress exists.
func findMostRecentIngress(route *routev1.Route) string {
	var newest string
	var recent time.Time
	for _, ingress := range route.Status.Ingress {
		if condition := findCondition(&ingress, routev1.RouteAdmitted); condition != nil && condition.LastTransitionTime != nil {
			if condition.LastTransitionTime.Time.After(recent) {
				recent = condition.LastTransitionTime.Time
				newest = ingress.RouterName
			}
		}
	}
	return newest
}

// findCondition locates the first condition that corresponds to the requested type.
func findCondition(ingress *routev1.RouteIngress, t routev1.RouteIngressConditionType) (_ *routev1.RouteIngressCondition) {
	for i, existing := range ingress.Conditions {
		if existing.Type != t {
			continue
		}
		return &ingress.Conditions[i]
	}
	return nil
}
