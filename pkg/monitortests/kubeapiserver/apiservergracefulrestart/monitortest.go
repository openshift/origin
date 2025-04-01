package apiservergracefulrestart

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type apiserverGracefulShutdownAnalyzer struct {
}

var (
	namespaceToServer = map[string]string{
		"openshift-kube-apiserver":  "kube-apiserver",
		"openshift-apiserver":       "openshift-apiserver",
		"openshift-oauth-apiserver": "oauth-apiserver",
	}
)

func NewGracefulShutdownAnalyzer() monitortestframework.MonitorTest {
	return &apiserverGracefulShutdownAnalyzer{}
}

func (w *apiserverGracefulShutdownAnalyzer) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *apiserverGracefulShutdownAnalyzer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *apiserverGracefulShutdownAnalyzer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*apiserverGracefulShutdownAnalyzer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	computedIntervals := monitorapi.Intervals{}

	startedIntervals := map[string]time.Time{}
	for _, currInterval := range startingIntervals {
		reason, isInteresting := interesting(currInterval)
		if !isInteresting {
			continue
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
		podRef := monitorapi.PodFrom(currInterval.Locator)
		nodeName, _ := currInterval.Locator.Keys[monitorapi.LocatorNodeKey]
		key := fmt.Sprintf("ns/%s pod/%s node/%s", podRef.Namespace, podRef.Name, nodeName)

		switch reason {
		case "ShutdownInitiated", "TerminationStart":
			if _, ok := startedIntervals[key]; !ok {
				startedIntervals[key] = currInterval.From
			}
		case "TerminationGracefulTerminationFinished":
			startTime := beginning
			if prevStart, ok := startedIntervals[key]; ok {
				startTime = prevStart
				delete(startedIntervals, key)
			}

			computedIntervals = append(computedIntervals,
				monitorapi.NewInterval(monitorapi.APIServerGracefulShutdown, monitorapi.Info).
					Locator(monitorapi.NewLocator().
						LocateServer(namespaceToServer[podRef.Namespace], nodeName, podRef.Namespace, podRef.Name),
					).
					Message(monitorapi.NewMessage().
						Constructed("graceful-shutdown-analyzer").
						Reason(monitorapi.GracefulAPIServerShutdown),
					).
					Display().
					Build(startTime, currInterval.To),
			)
		}
	}

	// and now close everything still open with a warning
	for fakeLocator, startTime := range startedIntervals {
		podRef := podFrom(fakeLocator)
		nodeName, _ := monitorapi.NodeFromLocator(fakeLocator)

		computedIntervals = append(computedIntervals,
			monitorapi.NewInterval(monitorapi.APIServerGracefulShutdown, monitorapi.Error).
				Locator(monitorapi.NewLocator().
					LocateServer(namespaceToServer[podRef.Namespace], nodeName, podRef.Namespace, podRef.Name),
				).
				Message(monitorapi.NewMessage().
					Constructed("graceful-shutdown-analyzer").
					Reason(monitorapi.IncompleteAPIServerShutdown),
				).
				Display().
				Build(startTime, time.Time{}),
		)
	}

	return computedIntervals, nil
}

// Deprecated: podFrom is a fork of the monitorapi.PodFrom function that was required due to the way this module is using
// keys in a map. Unfortunately we need to preserve this as structs do not work well in map keys, so we'd need a
// deeper refactor. For now, continue parsing locator info out of these strings in this one spot, but avoid
// use of this function.
func podFrom(locator string) monitorapi.PodReference {
	parts := monitorapi.LocatorParts(locator)
	namespace := monitorapi.NamespaceFrom(parts)
	name := parts[string(monitorapi.LocatorPodKey)]
	uid := parts[string(monitorapi.LocatorUIDKey)]
	if len(namespace) == 0 || len(name) == 0 {
		return monitorapi.PodReference{}
	}
	return monitorapi.PodReference{
		NamespacedReference: monitorapi.NamespacedReference{
			Namespace: namespace,
			Name:      name,
			UID:       uid,
		},
	}
}

func (*apiserverGracefulShutdownAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *apiserverGracefulShutdownAnalyzer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*apiserverGracefulShutdownAnalyzer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

func interesting(interval monitorapi.Interval) (monitorapi.IntervalReason, bool) {
	reason := interval.Message.Reason
	switch reason {
	// openshift-apiserver still is using the old event name TerminationStart
	case "ShutdownInitiated", "TerminationStart", "TerminationGracefulTerminationFinished":
		return reason, true
	default:
		return "", false
	}
}
