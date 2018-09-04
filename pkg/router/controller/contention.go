package controller

import (
	"sync"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	routev1 "github.com/openshift/api/route/v1"
)

// ContentionTracker records modifications to a particular entry to prevent endless
// loops when multiple routers are configured with conflicting info. A given router
// process tracks whether the ingress status is change from a correct value to any
// other value (by invoking IsContended when the state has diverged).
type ContentionTracker interface {
	// IsContended should be invoked when the state of the object in storage differs
	// from the desired state. It will return true if the provided id was recently
	// reset from the correct state to an incorrect state. The current ingress is the
	// expected state of the object at this time and may be used by the tracker to
	// determine if the most recent update was a contention. This method does not
	// update the state of the tracker.
	IsChangeContended(id string, now time.Time, current *routev1.RouteIngress) bool
	// Clear informs the tracker that the provided ingress state was confirmed to
	// match the current state of this process. If a subsequent call to IsChangeContended
	// is made within the expiration window, the object will be considered as contended.
	Clear(id string, current *routev1.RouteIngress)
}

type elementState int

const (
	stateCandidate elementState = iota
	stateContended
)

type trackerElement struct {
	at    time.Time
	state elementState
	last  *routev1.RouteIngress
}

// SimpleContentionTracker tracks whether a given identifier is changed from a correct
// state (set by Clear) to an incorrect state (inferred by calling IsContended).
type SimpleContentionTracker struct {
	informer   cache.SharedInformer
	routerName string

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
// growing unbounded if routes are created and deleted frequently. The informer
// detects changes to ingress records for routerName and will advance the tracker
// state from candidate to contended if the host, wildcardPolicy, or canonical host
// name fields are repeatedly updated.
func NewSimpleContentionTracker(informer cache.SharedInformer, routerName string, interval time.Duration) *SimpleContentionTracker {
	return &SimpleContentionTracker{
		informer:       informer,
		routerName:     routerName,
		expires:        interval,
		maxContentions: 5,

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
	// Watch the informer for changes to the route ingress we care about (identified
	// by router name) and if we see it change remember it. This loop can process
	// changes to routes faster than the router, which means it has more up-to-date
	// contention info and can detect contention while the main controller is still
	// syncing.
	t.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, obj interface{}) {
			oldRoute, ok := oldObj.(*routev1.Route)
			if !ok {
				return
			}
			route, ok := obj.(*routev1.Route)
			if !ok {
				return
			}
			if ingress := ingressChanged(oldRoute, route, t.routerName); ingress != nil {
				t.Changed(string(route.UID), ingress)
			}
		},
	})

	// periodically clean up expired changes
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
	contentionExpiration := nowFn().Add(-t.expires)
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

// Changed records that a change to an ingress value was detected. This is called from
// a separate goroutine and may have seen newer events than the current route controller
// plugins, so we don't do direct time comparisons. Instead we count edge transitions on
// a given id.
func (t *SimpleContentionTracker) Changed(id string, current *routev1.RouteIngress) {
	t.lock.Lock()
	defer t.lock.Unlock()

	// we have detected a sufficient number of conflicts to skip all updates for this interval
	if t.contentions > t.maxContentions {
		glog.V(4).Infof("Reached max contentions, stop tracking changes")
		return
	}

	// if we have never recorded this object
	last, ok := t.ids[id]
	if !ok {
		t.ids[id] = trackerElement{
			at:    nowFn().Time,
			state: stateCandidate,
			last:  current,
		}
		glog.V(4).Infof("Object %s is a candidate for contention", id)
		return
	}

	// the previous state matches the current state, nothing to do
	if ingressEqual(last.last, current) {
		glog.V(4).Infof("Object %s is unchanged", id)
		return
	}

	if last.state == stateContended {
		t.contentions++
		glog.V(4).Infof("Object %s is contended and has been modified by another writer", id)
		return
	}

	// if it appears that the state is being changed by another party, mark it as contended
	if last.state == stateCandidate {
		t.ids[id] = trackerElement{
			at:    nowFn().Time,
			state: stateContended,
			last:  current,
		}
		t.contentions++
		glog.V(4).Infof("Object %s has been modified by another writer", id)
		return
	}
}

func (t *SimpleContentionTracker) IsChangeContended(id string, now time.Time, current *routev1.RouteIngress) bool {
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
		return false
	}

	// if the object is contended, exit early
	if last.state == stateContended {
		glog.V(4).Infof("Object %s is being contended by another writer", id)
		return true
	}

	return false
}

func (t *SimpleContentionTracker) Clear(id string, current *routev1.RouteIngress) {
	t.lock.Lock()
	defer t.lock.Unlock()

	last, ok := t.ids[id]
	if !ok {
		return
	}
	last.last = current
	last.state = stateCandidate
	t.ids[id] = last
}

func ingressEqual(a, b *routev1.RouteIngress) bool {
	return a.Host == b.Host && a.RouterCanonicalHostname == b.RouterCanonicalHostname && a.WildcardPolicy == b.WildcardPolicy && a.RouterName == b.RouterName
}

func ingressConditionTouched(ingress *routev1.RouteIngress) *metav1.Time {
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

func ingressChanged(oldRoute, route *routev1.Route, routerName string) *routev1.RouteIngress {
	var ingress *routev1.RouteIngress
	for i := range route.Status.Ingress {
		if route.Status.Ingress[i].RouterName == routerName {
			ingress = &route.Status.Ingress[i]
			for _, old := range oldRoute.Status.Ingress {
				if old.RouterName == routerName {
					if !ingressEqual(ingress, &old) {
						return ingress
					}
					return nil
				}
			}
			return nil
		}
	}
	return nil
}
