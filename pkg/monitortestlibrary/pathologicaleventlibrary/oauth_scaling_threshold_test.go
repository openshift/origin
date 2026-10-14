package pathologicaleventlibrary

import (
	"fmt"
	"testing"

	v1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// These tests pin the scoped, job-wide tolerance for oauth-openshift ScalingReplicaSet repeats
// during the AuthenticationComponentProxy serial tests. The exception only raises how many
// repeats are tolerated for that specific namespace/deployment/reason while the proxy tests
// ran; it is not a causal fix and makes no claim the events are harmless.

// componentProxyE2EInterval is a completed AuthenticationComponentProxy e2e test interval; its
// presence is what registers the scoped oauth allowances (authenticationComponentProxyTestsDetected).
func componentProxyE2EInterval() monitorapi.Interval {
	return monitorapi.Interval{
		Source: monitorapi.SourceE2ETest,
		Condition: monitorapi.Condition{
			Locator: monitorapi.NewLocator().E2ETest(
				`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial] operator should validate OIDC IdP through component proxy with trustedCA`),
			Message: monitorapi.NewMessage().Reason(monitorapi.E2ETestFinished).
				WithAnnotation(monitorapi.AnnotationStatus, "Passed").Build(),
		},
	}
}

// kubeEventWithDeployment builds a KubeEvent interval for a deployment-scoped reason with a
// repetition count, matching how the legacy matcher identifies oauth scaling events.
func kubeEventWithDeployment(namespace, deployment, reason string, count int) monitorapi.Interval {
	return monitorapi.Interval{
		Source: monitorapi.SourceKubeEvent,
		Condition: monitorapi.Condition{
			Locator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeKind,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorNamespaceKey:  namespace,
					monitorapi.LocatorDeploymentKey: deployment,
				},
			},
			Message: monitorapi.Message{
				Reason:       monitorapi.IntervalReason(reason),
				HumanMessage: "(combined from similar events): Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3",
				Annotations: map[monitorapi.AnnotationKey]string{
					monitorapi.AnnotationCount: fmt.Sprintf("%d", count),
				},
			},
		},
	}
}

// TestAuthenticationComponentProxyScalingReplicaSet_Threshold70 covers the boundary of the
// scoped exception directly on the registered matcher.
func TestAuthenticationComponentProxyScalingReplicaSet_Threshold70(t *testing.T) {
	events := monitorapi.Intervals{componentProxyE2EInterval()}
	registry := NewUniversalPathologicalEventMatchers(nil, events)

	m, err := registry.GetMatcherByName("AuthenticationComponentProxyScalingReplicaSet")
	require.NoError(t, err)
	sm, ok := m.(*SimplePathologicalEventMatcher)
	require.True(t, ok)
	assert.Equal(t, 70, sm.repeatThresholdOverride, "oauth ScalingReplicaSet scoped tolerance must be 70")

	for _, count := range []int{56, 58, 63, 64, 70} {
		ev := kubeEventWithDeployment("openshift-authentication", "oauth-openshift", "ScalingReplicaSet", count)
		assert.Truef(t, m.Allows(ev, v1.HighlyAvailableTopologyMode), "count %d (<=70) must be allowed by the scoped exception", count)
	}
	over := kubeEventWithDeployment("openshift-authentication", "oauth-openshift", "ScalingReplicaSet", 71)
	assert.False(t, m.Allows(over, v1.HighlyAvailableTopologyMode), "count 71 (>70) must NOT be allowed")
}

// TestAuthenticationComponentProxyScalingReplicaSet_NegativeControls confirms the exception is
// tightly scoped: wrong deployment/reason are not matched, and absent the eligible feature test
// the exception is not registered at all.
func TestAuthenticationComponentProxyScalingReplicaSet_NegativeControls(t *testing.T) {
	events := monitorapi.Intervals{componentProxyE2EInterval()}
	registry := NewUniversalPathologicalEventMatchers(nil, events)
	m, err := registry.GetMatcherByName("AuthenticationComponentProxyScalingReplicaSet")
	require.NoError(t, err)

	// Wrong deployment in the same namespace: not the oauth-openshift rollout -> not allowed.
	wrongDeploy := kubeEventWithDeployment("openshift-authentication", "not-oauth-openshift", "ScalingReplicaSet", 30)
	assert.False(t, m.Allows(wrongDeploy, v1.HighlyAvailableTopologyMode), "non-oauth deployment must not be allowed")

	// Wrong reason on the oauth deployment: only ScalingReplicaSet is in scope.
	wrongReason := kubeEventWithDeployment("openshift-authentication", "oauth-openshift", "DeploymentUpdated", 30)
	assert.False(t, m.Allows(wrongReason, v1.HighlyAvailableTopologyMode), "non-ScalingReplicaSet reason must not be allowed")

	// Absent the eligible AuthenticationComponentProxy feature test, the scoped exception is not
	// even registered, so oauth scaling repeats are not tolerated.
	noFeatureRegistry := NewUniversalPathologicalEventMatchers(nil, monitorapi.Intervals{})
	_, err = noFeatureRegistry.GetMatcherByName("AuthenticationComponentProxyScalingReplicaSet")
	assert.Error(t, err, "without the eligible feature test the oauth scaling exception must be absent")
}

// TestAuthenticationComponentProxyDeploymentUpdated_Unchanged guards that the separate
// DeploymentUpdated allowance remains at 70 and is unaffected by this change.
func TestAuthenticationComponentProxyDeploymentUpdated_Unchanged(t *testing.T) {
	events := monitorapi.Intervals{componentProxyE2EInterval()}
	registry := NewUniversalPathologicalEventMatchers(nil, events)

	m, err := registry.GetMatcherByName("AuthenticationComponentProxyDeploymentUpdated")
	require.NoError(t, err)
	sm, ok := m.(*SimplePathologicalEventMatcher)
	require.True(t, ok)
	assert.Equal(t, 70, sm.repeatThresholdOverride, "DeploymentUpdated allowance must remain 70")

	at70 := kubeEventWithDeployment("openshift-authentication-operator", "authentication-operator", "DeploymentUpdated", 70)
	assert.True(t, m.Allows(at70, v1.HighlyAvailableTopologyMode), "DeploymentUpdated count 70 must remain allowed")
	at71 := kubeEventWithDeployment("openshift-authentication-operator", "authentication-operator", "DeploymentUpdated", 71)
	assert.False(t, m.Allows(at71, v1.HighlyAvailableTopologyMode), "DeploymentUpdated count 71 must remain rejected")
}

// TestAuthenticationComponentProxyScalingReplicaSet_ThroughLegacyConsumer drives the actual
// legacy evaluator (testDuplicatedEvents) to confirm the scoped tolerance's effect end-to-end:
// 70 repeats pass for ns/openshift-authentication, 71 rejects.
func TestAuthenticationComponentProxyScalingReplicaSet_ThroughLegacyConsumer(t *testing.T) {
	run := func(count int) *junitapiCaseResult {
		events := monitorapi.Intervals{
			componentProxyE2EInterval(),
			kubeEventWithDeployment("openshift-authentication", "oauth-openshift", "ScalingReplicaSet", count),
		}
		evaluator := duplicateEventsEvaluator{registry: NewUniversalPathologicalEventMatchers(nil, events)}
		junits := evaluator.testDuplicatedEvents("events should not repeat", false, events, nil, false)
		name := getJUnitName("events should not repeat", "openshift-authentication")
		res := &junitapiCaseResult{}
		for _, j := range junits {
			if j.Name == name && j.FailureOutput != nil {
				res.failed = true
				res.output = j.FailureOutput.Output
			}
		}
		return res
	}

	assert.False(t, run(70).failed, "70 oauth scaling repeats must pass through the legacy consumer")
	rejected := run(71)
	assert.True(t, rejected.failed, "71 oauth scaling repeats must be rejected by the legacy consumer")
	assert.Contains(t, rejected.output, "result=reject")
}

type junitapiCaseResult struct {
	failed bool
	output string
}
