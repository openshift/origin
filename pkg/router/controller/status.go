package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	client "github.com/openshift/origin/pkg/route/generated/internalclientset/typed/route/internalversion"
	"github.com/openshift/origin/pkg/router"
	"github.com/openshift/origin/pkg/util/writerlease"
)

// RejectionRecorder is an object capable of recording why a route was rejected
type RejectionRecorder interface {
	RecordRouteRejection(route *routeapi.Route, reason, message string)
}

// LogRejections writes rejection messages to the log.
var LogRejections = logRecorder{}

type logRecorder struct{}

func (logRecorder) RecordRouteRejection(route *routeapi.Route, reason, message string) {
	glog.V(3).Infof("Rejected route %s in namespace %s: %s: %s", route.Name, route.Namespace, reason, message)
}

// StatusAdmitter ensures routes added to the plugin have status set.
type StatusAdmitter struct {
	plugin                  router.Plugin
	client                  client.RoutesGetter
	routerName              string
	routerCanonicalHostname string

	lease   writerlease.Lease
	tracker ContentionTracker
}

// NewStatusAdmitter creates a plugin wrapper that ensures every accepted
// route has a status field set that matches this router. The admitter manages
// an LRU of recently seen conflicting updates to handle when two router processes
// with differing configurations are writing updates at the same time.
func NewStatusAdmitter(plugin router.Plugin, client client.RoutesGetter, name, hostName string, lease writerlease.Lease, tracker ContentionTracker) *StatusAdmitter {
	return &StatusAdmitter{
		plugin:                  plugin,
		client:                  client,
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
func (a *StatusAdmitter) HandleRoute(eventType watch.EventType, route *routeapi.Route) error {
	if IsGeneratedRouteName(route.Name) {
		// Can't record status for ingress resources
	} else {
		switch eventType {
		case watch.Added, watch.Modified:
			performIngressConditionUpdate("admit", a.lease, a.tracker, a.client, route, a.routerName, a.routerCanonicalHostname, routeapi.RouteIngressCondition{
				Type:   routeapi.RouteAdmitted,
				Status: kapi.ConditionTrue,
			})
		}
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
func (a *StatusAdmitter) RecordRouteRejection(route *routeapi.Route, reason, message string) {
	if IsGeneratedRouteName(route.Name) {
		// Can't record status for ingress resources
		return
	}
	performIngressConditionUpdate("reject", a.lease, a.tracker, a.client, route, a.routerName, a.routerCanonicalHostname, routeapi.RouteIngressCondition{
		Type:    routeapi.RouteAdmitted,
		Status:  kapi.ConditionFalse,
		Reason:  reason,
		Message: message,
	})
}

// performIngressConditionUpdate updates the route to the the appropriate status for the provided condition.
func performIngressConditionUpdate(action string, lease writerlease.Lease, tracker ContentionTracker, oc client.RoutesGetter, route *routeapi.Route, name, hostName string, condition routeapi.RouteIngressCondition) {
	key := string(route.UID)
	newestIngressName := findMostRecentIngress(route)
	changed, created, now, latest := recordIngressCondition(route, name, hostName, condition)
	if !changed {
		glog.V(4).Infof("%s: no changes to route needed: %s/%s", action, route.Namespace, route.Name)
		tracker.Clear(key, latest)
		// if the most recent change was to our ingress status, consider the current lease extended
		if newestIngressName == name {
			lease.Extend(key)
		}
		return
	}
	// If the tracker determines that another process is attempting to update the ingress to an inconsistent
	// value, skip updating altogether and rely on the next resync to resolve conflicts. This prevents routers
	// with different configurations from endlessly updating the route status.
	if !created && tracker.IsContended(key, now, latest) {
		glog.V(4).Infof("%s: skipped update due to another process altering the route with a different ingress status value: %s", action, key)
		return
	}

	lease.Try(key, func() (bool, bool) {
		updated := updateStatus(oc, route)
		if updated {
			tracker.Clear(key, latest)
		} else {
			glog.V(4).Infof("%s: did not update route status, skipping route until next resync", action)
		}
		return updated, false
	})
}

// recordIngressCondition updates the matching ingress on the route (or adds a new one) with the specified
// condition, returning whether the route was updated or created, the time assigned to the condition, and
// a pointer to the current ingress record.
func recordIngressCondition(route *routeapi.Route, name, hostName string, condition routeapi.RouteIngressCondition) (changed, created bool, _ time.Time, latest *routeapi.RouteIngress) {
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
			return false, false, time.Time{}, existing
		}

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

		return true, false, now.Time, existing
	}

	// add a new ingress
	route.Status.Ingress = append(route.Status.Ingress, routeapi.RouteIngress{
		RouterName:              name,
		Host:                    route.Spec.Host,
		WildcardPolicy:          route.Spec.WildcardPolicy,
		RouterCanonicalHostname: hostName,
		Conditions: []routeapi.RouteIngressCondition{
			condition,
		},
	})
	ingress := &route.Status.Ingress[len(route.Status.Ingress)-1]
	now := nowFn()
	ingress.Conditions[0].LastTransitionTime = &now

	return true, true, now.Time, ingress
}

// findMostRecentIngress returns the name of the ingress status with the most recent Admitted condition transition time,
// or an empty string if no such ingress exists.
func findMostRecentIngress(route *routeapi.Route) string {
	var newest string
	var recent time.Time
	for _, ingress := range route.Status.Ingress {
		if condition := findCondition(&ingress, routeapi.RouteAdmitted); condition != nil && condition.LastTransitionTime != nil {
			if condition.LastTransitionTime.Time.After(recent) {
				recent = condition.LastTransitionTime.Time
				newest = ingress.RouterName
			}
		}
	}
	return newest
}

// findCondition locates the first condition that corresponds to the requested type.
func findCondition(ingress *routeapi.RouteIngress, t routeapi.RouteIngressConditionType) (_ *routeapi.RouteIngressCondition) {
	for i, existing := range ingress.Conditions {
		if existing.Type != t {
			continue
		}
		return &ingress.Conditions[i]
	}
	return nil
}

func updateStatus(oc client.RoutesGetter, route *routeapi.Route) bool {
	for i := 0; i < 3; i++ {
		switch _, err := oc.Routes(route.Namespace).UpdateStatus(route); {
		case err == nil:
			return true
		case errors.IsNotFound(err):
			// route was deleted
			return false
		case errors.IsForbidden(err):
			// if the router can't write status updates, allow the route to go through
			utilruntime.HandleError(fmt.Errorf("Unable to write router status - please ensure you reconcile your system policy or grant this router access to update route status: %v", err))
			return true
		case errors.IsConflict(err):
			// just follow the normal process, and retry when we receive the update notification due to
			// the other entity updating the route.
			glog.V(4).Infof("updating status of %s/%s failed due to write conflict", route.Namespace, route.Name)
			return false
		default:
			utilruntime.HandleError(fmt.Errorf("Unable to write router status for %s/%s: %v", route.Namespace, route.Name, err))
			continue
		}
	}
	return false
}

// ContentionTracker records modifications to a particular entry to prevent endless
// loops when multiple routers are configured with conflicting info. A given router
// process tracks whether the ingress status is change from a correct value to any
// other value (by invoking IsContended when the state has diverged).
type ContentionTracker interface {
	// IsContended should be invoked when the state of the object in storage differs
	// from the desired state. It will return true if the provided id was recently
	// reset from the correct state to an incorrect state. The input ingress is the
	// expected state of the object at this time and may be used by the tracker to
	// determine if the most recent update was a contention. now is the current time
	// that should be used to record the change.
	IsContended(id string, now time.Time, ingress *routeapi.RouteIngress) bool
	// Clear informs the tracker that the provided ingress state was confirmed to
	// match the current state of this process. If a subsequent call to IsContended
	// is made within the expiration window, the object will be considered as contended.
	Clear(id string, ingress *routeapi.RouteIngress)
}

type elementState int

const (
	stateIncorrect elementState = iota
	stateCorrect
	stateContended
)

type trackerElement struct {
	at    time.Time
	state elementState
}

// SimpleContentionTracker tracks whether a given identifier is changed from a correct
// state (set by Clear) to an incorrect state (inferred by calling IsContended).
type SimpleContentionTracker struct {
	expires time.Duration
	// maxContentions is the number of contentions detected before the entire
	maxContentions int
	message        string

	lock        sync.Mutex
	contentions int
	ids         map[string]trackerElement
}

// NewSimpleContentionTracker creates a ContentionTracker that will prevent writing
// to the same route more often than once per interval. A background process will
// periodically flush old entries (at twice interval) in order to prevent the list
// growing unbounded if routes are created and deleted frequently.
func NewSimpleContentionTracker(interval time.Duration) *SimpleContentionTracker {
	return &SimpleContentionTracker{
		expires:        interval,
		maxContentions: 10,

		ids: make(map[string]trackerElement),
	}
}

// SetConflictMessage will print message whenever contention with another writer
// is detected.
func (t *SimpleContentionTracker) SetConflictMessage(message string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.message = message
}

// Run starts the background cleanup process for expired items.
func (t *SimpleContentionTracker) Run(stopCh <-chan struct{}) {
	ticker := time.NewTicker(t.expires * 2)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			t.flush()
		}
	}
}

func (t *SimpleContentionTracker) flush() {
	t.lock.Lock()
	defer t.lock.Unlock()

	// reset conflicts every expiration interval, but remove tracking info less often
	contentionExpiration := time.Now().Add(-t.expires)
	trackerExpiration := contentionExpiration.Add(-2 * t.expires)

	removed := 0
	contentions := 0
	for id, last := range t.ids {
		switch last.state {
		case stateContended:
			if last.at.Before(contentionExpiration) {
				delete(t.ids, id)
				removed++
				continue
			}
			contentions++
		default:
			if last.at.Before(trackerExpiration) {
				delete(t.ids, id)
				removed++
				continue
			}

		}
	}
	if t.contentions > 0 && len(t.message) > 0 {
		glog.Warning(t.message)
	}
	glog.V(5).Infof("Flushed contention tracker (%s): %d out of %d removed, %d total contentions", t.expires*2, removed, removed+len(t.ids), t.contentions)
	t.contentions = contentions
}

func (t *SimpleContentionTracker) IsContended(id string, now time.Time, ingress *routeapi.RouteIngress) bool {
	t.lock.Lock()
	defer t.lock.Unlock()

	// we have detected a sufficient number of conflicts to skip all updates for this interval
	if t.contentions > t.maxContentions {
		glog.V(4).Infof("Reached max contentions, rejecting all update attempts until the next interval")
		return true
	}

	// if we have expired or never recorded this object
	last, ok := t.ids[id]
	if !ok || last.at.Add(t.expires).Before(now) {
		t.ids[id] = trackerElement{
			at:    now,
			state: stateIncorrect,
		}
		return false
	}

	// if the object is contended, exit early
	if last.state == stateContended {
		glog.V(4).Infof("Object %s is being written to by another actor", id)
		return true
	}

	// if the object was previously correct, someone is contending with us
	if last.state == stateCorrect {
		t.ids[id] = trackerElement{
			at:    now,
			state: stateContended,
		}
		t.contentions++
		return true
	}

	return false
}

func (t *SimpleContentionTracker) Clear(id string, ingress *routeapi.RouteIngress) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if at := ingressConditionTouched(ingress); at != nil {
		t.ids[id] = trackerElement{
			at:    at.Time,
			state: stateCorrect,
		}
	}
}

func ingressConditionTouched(ingress *routeapi.RouteIngress) *metav1.Time {
	var lastTouch *metav1.Time
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
