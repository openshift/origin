package legacytestframeworkmonitortests

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/pathologicaleventlibrary"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

// TestEvaluateTestsFromConstructedIntervals_ShadowPreservesExistingVerdicts exercises the
// real production consumer path (the monitor test's EvaluateTestsFromConstructedIntervals)
// and proves that wiring the shadow accounting in does NOT change the existing
// "[sig-arch] events should not repeat pathologically" verdicts. It compares the monitor
// test output (minus the new informational shadow case) byte-for-byte with the output of the
// underlying evaluator the repository used before this change.
func TestEvaluateTestsFromConstructedIntervals_ShadowPreservesExistingVerdicts(t *testing.T) {
	base := time.Date(2026, 1, 2, 20, 0, 0, 0, time.UTC)
	prefix := `[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial]`

	// A passing eligible T4 window plus an oauth scaling event with two observations of the
	// same UID inside T4 (attributable in shadow), AND an unrelated unforgiven pathological
	// event in a core namespace so the existing core verdict is a real FAILURE. This ensures
	// we prove preservation of a failing verdict, not just an all-pass case.
	scaleMsg := "(combined from similar events): Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3"
	oauthA := pathologicaleventlibrary.BuildTestDupeKubeEvent("openshift-authentication", "oauth-openshift", "ScalingReplicaSet", scaleMsg, 5)
	oauthA.Locator.Keys[monitorapi.LocatorDeploymentKey] = "oauth-openshift"
	oauthA.Message.Annotations["eventUID"] = "uid-a"
	oauthA.Message.Annotations["resourceVersion"] = "100"
	oauthA.Message.Annotations["seriesLastObservedTime"] = base.Add(63 * time.Minute).Format(time.RFC3339)

	oauthB := pathologicaleventlibrary.BuildTestDupeKubeEvent("openshift-authentication", "oauth-openshift", "ScalingReplicaSet", scaleMsg, 8)
	oauthB.Locator.Keys[monitorapi.LocatorDeploymentKey] = "oauth-openshift"
	oauthB.Message.Annotations["eventUID"] = "uid-a"
	oauthB.Message.Annotations["resourceVersion"] = "101"
	oauthB.Message.Annotations["seriesLastObservedTime"] = base.Add(64 * time.Minute).Format(time.RFC3339)

	// Unforgiven pathological event in a known core namespace: must fail the core test.
	unforgiven := pathologicaleventlibrary.BuildTestDupeKubeEvent("openshift-etcd", "somepod", "SomethingUnusualHappened", "a message with no allowance", 40)

	t4 := monitorapi.NewInterval(monitorapi.SourceE2ETest, monitorapi.Info).
		Locator(monitorapi.NewLocator().E2ETest(prefix+" oauth-server should perform full OIDC login flow through the proxy when auth proxy config is applied")).
		Message(monitorapi.NewMessage().HumanMessage("e2e test finished").WithAnnotation(monitorapi.AnnotationStatus, "Passed")).
		Build(base.Add(62*time.Minute), base.Add(67*time.Minute))

	intervals := monitorapi.Intervals{t4, oauthA, oauthB, unforgiven}

	// Baseline: exactly what the repository evaluated before this change (stable system path,
	// nil kube config as in an offline evaluation).
	baseline := pathologicaleventlibrary.TestDuplicatedEventForStableSystem(intervals, nil)

	mt := NewLegacyPathologicalMonitorTests(monitortestframework.MonitorTestInitializationInfo{})
	got, err := mt.EvaluateTestsFromConstructedIntervals(context.TODO(), intervals)
	require.NoError(t, err)

	var shadow *junitapi.JUnitTestCase
	var gotWithoutShadow []*junitapi.JUnitTestCase
	for _, j := range got {
		if strings.Contains(j.Name, "bounded accounting (shadow)") {
			shadow = j
			continue
		}
		gotWithoutShadow = append(gotWithoutShadow, j)
	}

	// The shadow case must exist and must never fail the run.
	require.NotNil(t, shadow, "shadow accounting JUnit should be present")
	assert.Nil(t, shadow.FailureOutput, "shadow JUnit must never carry a failure")

	// Exact preservation: removing the shadow case yields precisely the pre-change output.
	// The underlying evaluator emits per-namespace cases in map-iteration order, so compare as
	// an unordered multiset (ElementsMatch deep-compares each case's content).
	require.Equal(t, len(baseline), len(gotWithoutShadow), "shadow must add exactly one JUnit case and change nothing else")
	assert.ElementsMatch(t, baseline, gotWithoutShadow, "existing pathological verdicts must be preserved exactly")

	// Sanity: the core namespace verdict is a real failure and remains a failure.
	var coreFailure bool
	for _, j := range gotWithoutShadow {
		if j.Name == "[sig-arch] events should not repeat pathologically for ns/openshift-etcd" && j.FailureOutput != nil {
			coreFailure = true
		}
	}
	assert.True(t, coreFailure, "the unforgiven core pathological event must still fail, unaffected by shadow")
}
