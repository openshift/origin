package synthetictests

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/test/ginkgo"
)

// stableSystemEventInvariants are invariants that should hold true when a cluster is in
// steady state (not being changed externally). Use these with suites that assume the
// cluster is under no adversarial change (config changes, induced disruption to nodes,
// etcd, or apis).
func StableSystemEventInvariants(events monitorapi.Intervals, duration time.Duration) (tests []*ginkgo.JUnitTestCase) {
	tests = SystemEventInvariants(events, duration)
	tests = append(tests, testContainerFailures(events)...)
	tests = append(tests, testDeleteGracePeriodZero(events)...)
	tests = append(tests, testKubeAPIServerGracefulTermination(events)...)
	tests = append(tests, testKubeletToAPIServerGracefulTermination(events)...)
	tests = append(tests, testPodTransitions(events)...)
	tests = append(tests, testPodSandboxCreation(events)...)
	tests = append(tests, testServerAvailability(monitor.LocatorKubeAPIServerNewConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorOpenshiftAPIServerNewConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorOAuthAPIServerNewConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorKubeAPIServerReusedConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorOpenshiftAPIServerReusedConnection, events, duration)...)
	tests = append(tests, testServerAvailability(monitor.LocatorOAuthAPIServerReusedConnection, events, duration)...)
	tests = append(tests, testStableSystemOperatorStateTransitions(events)...)

	return tests
}

// systemUpgradeEventInvariants are invariants tested against events that should hold true in a cluster
// that is being upgraded without induced disruption
func SystemUpgradeEventInvariants(events monitorapi.Intervals, duration time.Duration) (tests []*ginkgo.JUnitTestCase) {
	tests = SystemEventInvariants(events, duration)
	tests = append(tests, testContainerFailures(events)...)
	tests = append(tests, testDeleteGracePeriodZero(events)...)
	tests = append(tests, testKubeAPIServerGracefulTermination(events)...)
	tests = append(tests, testKubeletToAPIServerGracefulTermination(events)...)
	tests = append(tests, testPodTransitions(events)...)
	tests = append(tests, testPodSandboxCreation(events)...)
	tests = append(tests, testNodeUpgradeTransitions(events)...)
	tests = append(tests, testUpgradeOperatorStateTransitions(events)...)
	return tests
}

// systemEventInvariants are invariants tested against events that should hold true in any cluster,
// even one undergoing disruption. These are usually focused on things that must be true on a single
// machine, even if the machine crashes.
func SystemEventInvariants(events monitorapi.Intervals, duration time.Duration) (tests []*ginkgo.JUnitTestCase) {
	tests = append(tests, testSystemDTimeout(events)...)
	return tests
}
