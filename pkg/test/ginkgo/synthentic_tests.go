package ginkgo

import (
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// JUnitsForEvents returns a set of JUnit results for the provided events encountered
// during a test suite run.
type JUnitsForEvents interface {
	// JUnitsForEvents returns a set of additional test passes or failures implied by the
	// events sent during the test suite run. If passed is false, the entire suite is failed.
	// To set a test as flaky, return a passing and failing JUnitTestCase with the same name.
	JUnitsForEvents(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string, recordedResource *monitorapi.ResourcesMap) []*junitapi.JUnitTestCase
}

// JUnitForEventsFunc converts a function into the JUnitForEvents interface.
// kubeClientConfig may or may not be present.  The JUnit evaluation needs to tolerate a missing *rest.Config
// and an unavailable cluster without crashing.
type JUnitForEventsFunc func(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string, recordedResource *monitorapi.ResourcesMap) []*junitapi.JUnitTestCase

func (fn JUnitForEventsFunc) JUnitsForEvents(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string, recordedResource *monitorapi.ResourcesMap) []*junitapi.JUnitTestCase {
	return fn(events, duration, kubeClientConfig, testSuite, recordedResource)
}

// JUnitsForAllEvents aggregates multiple JUnitsForEvent interfaces and returns
// the result of all invocations. It ignores nil interfaces.
type JUnitsForAllEvents []JUnitsForEvents

func (a JUnitsForAllEvents) JUnitsForEvents(events monitorapi.Intervals, duration time.Duration, kubeClientConfig *rest.Config, testSuite string, recordedResource *monitorapi.ResourcesMap) []*junitapi.JUnitTestCase {
	var all []*junitapi.JUnitTestCase
	for _, obj := range a {
		if obj == nil {
			continue
		}
		results := obj.JUnitsForEvents(events, duration, kubeClientConfig, testSuite, recordedResource)
		all = append(all, results...)
	}
	return all
}
