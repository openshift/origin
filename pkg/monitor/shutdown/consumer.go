package shutdown

import (
	"fmt"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/kubernetes/test/e2e/framework"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	Reason                   = "GracefulShutdownWindow"
	APIServerShutdownLocator = "shutdown/apiserver"
)

var (
	namespaceToServer = map[string]string{
		"openshift-kube-apiserver": "kube-apiserver",
		"openshift-apiserver":      "openshift-apiserver",
		// TODO: oauth apiserver does not seem to emit any shutdown events
		"openshift-oauth-apiserver": "oauth-apiserver",
	}
)

// Monitor abstracts the monitor API
type Monitor interface {
	StartInterval(t time.Time, condition monitorapi.Condition) int
	EndInterval(startedInterval int, t time.Time)
}

func newConsumer(monitor Monitor) *consumer {
	return &consumer{
		monitor:   monitor,
		byHost:    make(map[string]*corev1.Event),
		processed: make(map[types.UID]struct{}),
	}
}

type consumer struct {
	monitor   Monitor
	lock      sync.Mutex
	processed map[types.UID]struct{}
	byHost    map[string]*corev1.Event
}

func (c *consumer) Consume(event *corev1.Event) {
	if event == nil || !interesting(event) {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	var ok bool
	if _, ok := c.processed[event.UID]; ok {
		return
	}

	// NOTE: we are working with the following constraints:
	//  a) we use the combination of namespace and pod name as the unique
	//     key to identify a [start, end] pair that represents a shutdown window.
	// 	b) pod names, as they appear in the events, are not expected to change
	//     for kube-apiserver, but it's not true for openshift-apiserver.
	//	c) we expect both start and end shutdown events for a certain host
	//     to appear and, do so in the desired order.
	//  d) we do not expect nested shutdown window(s), for a certain host:
	//      startW1, startW2, endW2, endW1
	//  e) the apiserver creates the start and end shutdown events in order
	//     with a significant delay in between, so it is highly unlikely that
	//     the end event will be stored first in etcd, but it is possible that
	//     creation of an event might fail and we see only one event from the pair.
	// Links:
	// a) https://github.com/openshift/kubernetes/blob/15f19ea2dd700767e5337502aec753d2a6e26905/staging/src/k8s.io/apiserver/pkg/server/config.go#L711-L742
	// e) start: https://github.com/openshift/kubernetes/blob/15f19ea2dd700767e5337502aec753d2a6e26905/staging/src/k8s.io/apiserver/pkg/server/genericapiserver.go#L547
	//    end: https://github.com/openshift/kubernetes/blob/15f19ea2dd700767e5337502aec753d2a6e26905/staging/src/k8s.io/apiserver/pkg/server/genericapiserver.go#L733
	//
	// One way to make this more deterministic would be to add a new carry in
	// o/k with the following change:
	//  - after we create the start event, save an object reference to this
	//    start event just created.
	//  - when we create the end event, set 'Related' field of the end event
	//    to that of the start event.
	// TODO: With the above approach, finding the pair will be more deterministic
	key := key(event)
	var start, end *corev1.Event
	start, ok = c.byHost[key]
	if !ok {
		c.byHost[key] = event
		return
	}
	end = event

	delete(c.byHost, key)
	c.processed[start.UID] = struct{}{}
	c.processed[end.UID] = struct{}{}

	condition := complete(start, end)
	// these logs can be useful to debug if the windows are not displayed correctly
	framework.Logf("GracefulShutdownEvent - Start: %+v", start)
	framework.Logf("GracefulShutdownEvent - End: %+v", end)
	framework.Logf(condition.Message)
	intervalID := c.monitor.StartInterval(timeOf(start), condition)
	c.monitor.EndInterval(intervalID, timeOf(end))
}

func (c *consumer) Done() {
	c.lock.Lock()
	defer c.lock.Unlock()

	// do we have any incomplete shutdown window?
	for key, event := range c.byHost {
		delete(c.byHost, key)

		startedAt := timeOf(event)
		condition := incomplete(event)
		framework.Logf("GracefulShutdownEvent - Incomplete: %+v", event)
		framework.Logf(condition.Message)
		intervalID := c.monitor.StartInterval(startedAt, condition)
		c.monitor.EndInterval(intervalID, startedAt.Add(time.Second))
	}
}

func key(event *corev1.Event) string {
	// we use the pod namespace/name as the unique key, this ensures that:
	// for a static pod, we can identify shutdown window(s) uniquely for each
	// apiserver.
	// for regular pod like openshift apiserver, we expect the key to identify
	// the shutdown window uniquely within the lifetime of the process.
	return fmt.Sprintf("%s/%s", event.InvolvedObject.Namespace, event.InvolvedObject.Name)
}

func interesting(event *corev1.Event) bool {
	switch event.Reason {
	// openshift-apiserver still is using the old event name TerminationStart
	case "ShutdownInitiated", "TerminationStart", "TerminationGracefulTerminationFinished":
		return true
	default:
		return false
	}
}

func timeOf(event *corev1.Event) time.Time {
	// this is the time that is populated for these events.
	return event.CreationTimestamp.Time
}

func namespaceOf(event *corev1.Event) string {
	// this is the time that is populated for these events.
	return event.InvolvedObject.Namespace
}

func nameOf(event *corev1.Event) string {
	// this is the time that is populated for these events.
	return event.InvolvedObject.Name
}

func hostOf(event *corev1.Event) string {
	return event.Source.Host
}

func complete(start, end *corev1.Event) monitorapi.Condition {
	locator := fmt.Sprintf("%s server/%s", APIServerShutdownLocator, namespaceToServer[namespaceOf(start)])
	message := fmt.Sprintf("shutdown window: name=%s namespace=%s host=%s duration=%s",
		nameOf(start), namespaceOf(start), hostOf(start), timeOf(end).Sub(timeOf(start)).Round(time.Second))
	return monitorapi.Condition{
		Level:   monitorapi.Info,
		Locator: locator,
		Message: fmt.Sprintf("reason/%s locator/%s : %s", Reason, locator, message),
	}
}

func incomplete(event *corev1.Event) monitorapi.Condition {
	locator := fmt.Sprintf("%s server/%s", APIServerShutdownLocator, namespaceToServer[event.Namespace])
	message := fmt.Sprintf("missing complementary event: reason=%s name=%s namespace=%s host=%s",
		event.Reason, nameOf(event), namespaceOf(event), hostOf(event))
	return monitorapi.Condition{
		Level:   monitorapi.Error,
		Locator: locator,
		Message: fmt.Sprintf("reason/%s locator/%s : %s", Reason, locator, message),
	}
}
