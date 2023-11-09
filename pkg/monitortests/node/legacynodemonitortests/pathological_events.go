package legacynodemonitortests

import (
	"math"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/pathologicaleventlibrary"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

func testNodeHasNoDiskPressure(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] pathological event NodeHasNoDiskPressure condition does not occur too often"
	return pathologicaleventlibrary.EventExprMatchThresholdTest(testName, events, pathologicaleventlibrary.NodeHasNoDiskPressure,
		pathologicaleventlibrary.DuplicateEventThreshold)
}

func testNodeHasSufficientMemory(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] pathological event NodeHasSufficeintMemory condition does not occur too often"
	return pathologicaleventlibrary.EventExprMatchThresholdTest(testName, events, pathologicaleventlibrary.NodeHasSufficientMemory, pathologicaleventlibrary.DuplicateEventThreshold)
}

func testNodeHasSufficientPID(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] pathological event NodeHasSufficientPID condition does not occur too often"
	return pathologicaleventlibrary.EventExprMatchThresholdTest(testName, events, pathologicaleventlibrary.NodeHasSufficientPID,
		pathologicaleventlibrary.DuplicateEventThreshold)
}

// testBackoffPullingRegistryRedhatImage looks for this symptom:
//
//	reason/ContainerWait ... Back-off pulling image "registry.redhat.io/openshift4/ose-oauth-proxy:latest"
//	reason/BackOff Back-off pulling image "registry.redhat.io/openshift4/ose-oauth-proxy:latest"
//
// to happen over a certain threshold and marks it as a failure or flake accordingly.
func testBackoffPullingRegistryRedhatImage(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	testName := "[sig-arch] pathological event should not see excessive pull back-off on registry.redhat.io"
	return pathologicaleventlibrary.NewSingleEventThresholdCheck(testName,
		pathologicaleventlibrary.AllowImagePullFromRedHatRegistry, math.MaxInt,
		pathologicaleventlibrary.ImagePullRedhatFlakeThreshold).Test(events)
}

// testBackoffStartingFailedContainer looks for this symptom in core namespaces:
//
//	reason/BackOff Back-off restarting failed container
func testBackoffStartingFailedContainer(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	testName := "[sig-cluster-lifecycle] pathological event should not see excessive Back-off restarting failed containers"

	return pathologicaleventlibrary.NewSingleEventThresholdCheck(testName, pathologicaleventlibrary.AllowBackOffRestartingFailedContainer,
		pathologicaleventlibrary.DuplicateEventThreshold, pathologicaleventlibrary.BackoffRestartingFlakeThreshold).
		Test(events.Filter(monitorapi.Not(monitorapi.IsInE2ENamespace)))
}

func testConfigOperatorReadinessProbe(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] pathological event openshift-config-operator readiness probe should not fail due to timeout"
	return pathologicaleventlibrary.MakeProbeTest(testName, events, "openshift-config-operator", pathologicaleventlibrary.ConfigOperatorProbeErrorReadiness, pathologicaleventlibrary.DuplicateEventThreshold)
}

func testConfigOperatorProbeErrorReadinessProbe(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] pathological event openshift-config-operator should not get probe error on readiness probe due to timeout"
	return pathologicaleventlibrary.MakeProbeTest(testName, events, "openshift-config-operator", pathologicaleventlibrary.ConfigOperatorProbeErrorReadiness, pathologicaleventlibrary.DuplicateEventThreshold)
}

func testConfigOperatorProbeErrorLivenessProbe(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] pathological event openshift-config-operator should not get probe error on liveness probe due to timeout"
	return pathologicaleventlibrary.MakeProbeTest(testName, events, "openshift-config-operator", pathologicaleventlibrary.ConfigOperatorProbeErrorLiveness, pathologicaleventlibrary.DuplicateEventThreshold)
}

func testFailedScheduling(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] pathological event FailedScheduling condition does not occur too often"
	return pathologicaleventlibrary.EventExprMatchThresholdTest(testName, events, pathologicaleventlibrary.FailedScheduling, pathologicaleventlibrary.DuplicateEventThreshold)
}
