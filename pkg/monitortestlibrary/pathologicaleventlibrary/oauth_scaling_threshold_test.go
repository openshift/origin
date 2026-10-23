package pathologicaleventlibrary

import (
	"fmt"
	"testing"
	"time"

	v1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// These tests pin the scoped, proximity-bounded tolerance for oauth-openshift ScalingReplicaSet
// repeats during the AuthenticationComponentProxy serial tests. The finite cap (70) applies
// only to repeats whose LATEST occurrence falls during one of the eligible passing tests or
// within authenticationComponentProxyPostTestGrace AFTER that test ends (no pre-test grace). It is a bounded proximity allowance, not
// a causal claim that the tests cause the rollouts, and not a claim the events are harmless.

const cpPrefix = `[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial]`

// The exact eligible test full names (prefix + " " + suffix). T3 (hot-reload) is an
// authorized eligible test by design. The noProxy bypass test is also eligible even though it
// directly connects to the IdP, bypassing the proxy.
var cpEligibleFullNames = []string{
	cpPrefix + " operator should validate OIDC IdP through component proxy with trustedCA",
	cpPrefix + " operator should validate OIDC IdP through component proxy",
	cpPrefix + " oauth-server/operator should hot-reload mounted CA file on change when spec.proxy.trustedCA is set",
	cpPrefix + " oauth-server should perform full OIDC login flow through the proxy when auth proxy config is applied",
	cpPrefix + " operator should fall back to original configuration on spec.proxy removal",
	cpPrefix + " oauth-server should bypass proxy by directly connecting to idp to perform OIDC login flow when spec.proxy.noProxy contains idp",
}

func cpTestInterval(fullName, status string, from, to time.Time) monitorapi.Interval {
	return monitorapi.Interval{
		Source: monitorapi.SourceE2ETest,
		Condition: monitorapi.Condition{
			Locator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeE2ETest,
				Keys: map[monitorapi.LocatorKey]string{monitorapi.LocatorE2ETestKey: fullName},
			},
			Message: monitorapi.Message{
				Reason:      monitorapi.E2ETestFinished,
				Annotations: map[monitorapi.AnnotationKey]string{monitorapi.AnnotationStatus: status},
			},
		},
		From: from,
		To:   to,
	}
}

// oauthScaleEvent builds an oauth-openshift ScalingReplicaSet KubeEvent whose LATEST occurrence
// is `latest`. Pathological-event intervals set From = LastTimestamp and To = From+1s (the
// second-precision cover); the proximity matcher uses those (useLastTimestamp=true).
func oauthScaleEvent(count int, latest time.Time) monitorapi.Interval {
	return oauthScaleEventNS("openshift-authentication", "oauth-openshift", "ScalingReplicaSet", count, latest)
}

func oauthScaleEventNS(namespace, deployment, reason string, count int, latest time.Time) monitorapi.Interval {
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
					// firstTimestamp intentionally much earlier than any window, to mirror the real
					// aggregated Event whose lifetime spans tests/gaps; useLastTimestamp=true must
					// ignore it and use the latest occurrence only.
					"firstTimestamp": latest.Add(-1 * time.Hour).Format(time.RFC3339),
					"lastTimestamp":  latest.Format(time.RFC3339),
				},
			},
		},
		From: latest,
		To:   latest.Add(1 * time.Second),
	}
}

func scalingMatcher(t *testing.T, events monitorapi.Intervals) EventMatcher {
	t.Helper()
	reg := NewUniversalPathologicalEventMatchers(nil, events)
	m, err := reg.GetMatcherByName("AuthenticationComponentProxyScalingReplicaSet")
	require.NoError(t, err)
	return m
}

// base window: [t0, t0+4m]; events placed relative to it.
var cpT0 = time.Date(2026, 1, 2, 1, 0, 0, 0, time.UTC)

func onePassingWindow() (monitorapi.Intervals, time.Time, time.Time) {
	from := cpT0
	to := cpT0.Add(4 * time.Minute)
	return monitorapi.Intervals{cpTestInterval(cpEligibleFullNames[0], "Passed", from, to)}, from, to
}

func TestAuthenticationComponentProxyScalingReplicaSet_EligibleDuringEachTest(t *testing.T) {
	// Every one of the six names (including the T3 hot-reload no-rollout test) authorizes the
	// tolerance for an in-window repeat at the cap.
	for i, name := range cpEligibleFullNames {
		from := cpT0.Add(time.Duration(i) * 10 * time.Minute)
		to := from.Add(4 * time.Minute)
		events := monitorapi.Intervals{cpTestInterval(name, "Passed", from, to)}
		m := scalingMatcher(t, events)
		inWindow := oauthScaleEvent(70, from.Add(2*time.Minute))
		assert.Truef(t, m.Allows(inWindow, v1.HighlyAvailableTopologyMode),
			"count 70 during eligible test %d must be allowed", i+1)
	}
}

func TestAuthenticationComponentProxyScalingReplicaSet_PostTestGraceAndBoundaries(t *testing.T) {
	grace := authenticationComponentProxyPostTestGrace
	events, from, to := onePassingWindow()
	m := scalingMatcher(t, events)

	cases := []struct {
		name   string
		latest time.Time
		allow  bool
	}{
		{"inside window", from.Add(2 * time.Minute), true},
		{"exact window end", to, true},
		{"29s after end (observed real offset)", to.Add(29 * time.Second), true},
		{"90s after end (within grace)", to.Add(90 * time.Second), true},
		{"150s after end (within grace)", to.Add(150 * time.Second), true},
		{"cover strictly within grace", to.Add(grace - 2*time.Second), true},
		{"cover ends at grace boundary", to.Add(grace - time.Second), false},
		{"exact grace end", to.Add(grace), false},
		{"before test start", from.Add(-10 * time.Second), false},
		{"30s past grace", to.Add(grace + 30*time.Second), false},
	}
	for _, c := range cases {
		got := m.Allows(oauthScaleEvent(70, c.latest), v1.HighlyAvailableTopologyMode)
		assert.Equalf(t, c.allow, got, "case %q", c.name)
	}
}

func TestAuthenticationComponentProxyScalingReplicaSet_SecondPrecisionGraceBoundary(t *testing.T) {
	grace := authenticationComponentProxyPostTestGrace
	events, _, to := onePassingWindow()
	m := scalingMatcher(t, events)

	// The latest occurrence's whole second-precision cover [t, t+1s) must fit within the
	// post-grace window; an occurrence whose cover extends past end+grace is NOT allowed, avoiding
	// the earlier unsafe point assumption that accepted +500ms past the end.
	wellWithin := to.Add(grace - time.Minute)
	assert.True(t, m.Allows(oauthScaleEvent(70, wellWithin), v1.HighlyAvailableTopologyMode),
		"1m before grace end: whole [t,t+1s) cover fits within the grace")

	// end+grace-0.5s: cover is [end+grace-0.5s, end+grace+0.5s), which extends past end+grace -> not allowed.
	straddles := to.Add(grace - 500*time.Millisecond)
	assert.False(t, m.Allows(oauthScaleEvent(70, straddles), v1.HighlyAvailableTopologyMode),
		"cover straddling the grace boundary -> conservatively rejected")
}

func TestAuthenticationComponentProxyScalingReplicaSet_OverCapRejectedEvenInWindow(t *testing.T) {
	events, from, to := onePassingWindow()
	m := scalingMatcher(t, events)
	over := oauthScaleEvent(71, from.Add(2*time.Minute))
	assert.False(t, m.Allows(over, v1.HighlyAvailableTopologyMode),
		"count 71 must be rejected even inside an eligible window")
	// The cap is enforced across the whole eligibility window, including the post-test grace.
	overInGrace := oauthScaleEvent(71, to.Add(90*time.Second))
	assert.False(t, m.Allows(overInGrace, v1.HighlyAvailableTopologyMode),
		"count 71 must be rejected even within the post-test grace")
}

func TestAuthenticationComponentProxyScalingReplicaSet_NegativeControls(t *testing.T) {
	events, from, _ := onePassingWindow()
	m := scalingMatcher(t, events)
	in := from.Add(2 * time.Minute)

	assert.False(t, m.Allows(oauthScaleEventNS("openshift-authentication", "not-oauth-openshift", "ScalingReplicaSet", 30, in), v1.HighlyAvailableTopologyMode),
		"wrong deployment not allowed")
	assert.False(t, m.Allows(oauthScaleEventNS("openshift-authentication", "oauth-openshift", "DeploymentUpdated", 30, in), v1.HighlyAvailableTopologyMode),
		"wrong reason not allowed")
	assert.False(t, m.Allows(oauthScaleEventNS("openshift-other", "oauth-openshift", "ScalingReplicaSet", 30, in), v1.HighlyAvailableTopologyMode),
		"wrong namespace not allowed")

	// Without eligible test intervals, the matcher is not registered and grants no exception.
	withoutTests := NewUniversalPathologicalEventMatchers(nil, nil)
	matcher, err := withoutTests.GetMatcherByName("AuthenticationComponentProxyScalingReplicaSet")
	assert.Error(t, err)
	assert.Nil(t, matcher)
	allowed, _ := withoutTests.AllowedByAny(oauthScaleEvent(70, in), v1.HighlyAvailableTopologyMode)
	assert.False(t, allowed,
		"without an eligible test the oauth scaling exception must not apply")
}

func TestAuthenticationComponentProxyScalingReplicaSet_FailedOrSkippedWindowExcluded(t *testing.T) {
	from := cpT0
	to := cpT0.Add(4 * time.Minute)
	for _, status := range []string{"Failed", "Skipped", "DidNotFinish"} {
		// A non-Passed eligible test yields no eligible window, so the matcher is not even
		// registered and grants no exception.
		events := monitorapi.Intervals{cpTestInterval(cpEligibleFullNames[0], status, from, to)}
		reg := NewUniversalPathologicalEventMatchers(nil, events)
		inWindow := oauthScaleEvent(70, from.Add(2*time.Minute))
		allowed, _ := reg.AllowedByAny(inWindow, v1.HighlyAvailableTopologyMode)
		assert.Falsef(t, allowed, "status %q must NOT grant the exception", status)
	}
}

func TestAuthenticationComponentProxyScalingReplicaSet_AdjacentWindowGapNotAllowed(t *testing.T) {
	// Two eligible windows with a gap. An event whose LATEST occurrence lands in the gap (past
	// the first window's grace, before the second starts) is not allowed, even though its earlier
	// lifetime overlapped the first window. Mirrors the real count-64 case.
	wA0 := cpT0
	wA1 := cpT0.Add(4 * time.Minute)
	wB0 := cpT0.Add(20 * time.Minute)
	wB1 := wB0.Add(4 * time.Minute)
	events := monitorapi.Intervals{
		cpTestInterval(cpEligibleFullNames[0], "Passed", wA0, wA1),
		cpTestInterval(cpEligibleFullNames[3], "Passed", wB0, wB1),
	}
	m := scalingMatcher(t, events)
	inGap := oauthScaleEvent(70, wA1.Add(11*time.Minute)) // well past A+grace, before B
	assert.False(t, m.Allows(inGap, v1.HighlyAvailableTopologyMode),
		"latest occurrence in the inter-test gap must not be allowed despite earlier overlap")
}

func TestAuthenticationComponentProxyScalingReplicaSet_DegenerateWindowExcluded(t *testing.T) {
	// Fail-closed: a test that reports Passed but has a missing or non-positive span (zero
	// timestamps, or To not strictly after From) yields no eligible window, so the matcher is
	// not even registered and grants no exception.
	zero := time.Time{}
	cases := []struct {
		name     string
		from, to time.Time
	}{
		{"zero from and to", zero, zero},
		{"zero to only", cpT0, zero},
		{"to equals from", cpT0, cpT0},
		{"to before from", cpT0.Add(time.Minute), cpT0},
	}
	for _, c := range cases {
		events := monitorapi.Intervals{cpTestInterval(cpEligibleFullNames[0], "Passed", c.from, c.to)}
		reg := NewUniversalPathologicalEventMatchers(nil, events)
		// Probe near cpT0 where a real window would have been.
		ev := oauthScaleEvent(70, cpT0.Add(2*time.Minute))
		allowed, _ := reg.AllowedByAny(ev, v1.HighlyAvailableTopologyMode)
		assert.Falsef(t, allowed, "degenerate window %q must grant no exception", c.name)
	}
}

// The operator event allowances apply only during eligible passing tests and their
// post-test grace, with separate caps for DeploymentUpdated and OperatorStatusChanged.
func TestAuthenticationComponentProxyOperatorEvents(t *testing.T) {
	grace := authenticationComponentProxyPostTestGrace
	events, from, to := onePassingWindow()
	reg := NewUniversalPathologicalEventMatchers(nil, events)
	for _, event := range []struct {
		reason string
		cap    int
	}{
		{reason: "DeploymentUpdated", cap: 70},
		{reason: "OperatorStatusChanged", cap: 30},
	} {
		t.Run(event.reason, func(t *testing.T) {
			m, err := reg.GetMatcherByName("AuthenticationComponentProxy" + event.reason)
			require.NoError(t, err)
			for _, tc := range []struct {
				name   string
				count  int
				latest time.Time
				allow  bool
			}{
				{"at cap during test", event.cap, from.Add(time.Minute), true},
				{"over cap during test", event.cap + 1, from.Add(time.Minute), false},
				{"at cap within grace", event.cap, to.Add(grace - time.Minute), true},
				{"over cap within grace", event.cap + 1, to.Add(grace - time.Minute), false},
				{"before test", event.cap, from.Add(-time.Second), false},
				{"past grace", event.cap, to.Add(grace + 30*time.Second), false},
				{"straddling grace boundary", event.cap, to.Add(grace - 500*time.Millisecond), false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					ev := oauthScaleEventNS("openshift-authentication-operator", "authentication-operator", event.reason, tc.count, tc.latest)
					assert.Equal(t, tc.allow, m.Allows(ev, v1.HighlyAvailableTopologyMode))
				})
			}
		})
	}
}

// TestAuthenticationComponentProxyScalingReplicaSet_ThroughLegacyConsumer drives the actual
// legacy evaluator end-to-end: an in-window 70 passes, 71 rejects, and a 70 past the grace
// rejects.
func TestAuthenticationComponentProxyScalingReplicaSet_ThroughLegacyConsumer(t *testing.T) {
	from := cpT0
	to := cpT0.Add(4 * time.Minute)
	win := cpTestInterval(cpEligibleFullNames[0], "Passed", from, to)

	run := func(ev monitorapi.Interval) *junitapiCaseResult {
		events := monitorapi.Intervals{win, ev}
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

	assert.False(t, run(oauthScaleEvent(70, from.Add(2*time.Minute))).failed, "in-window 70 must pass the legacy consumer")
	assert.False(t, run(oauthScaleEvent(70, to.Add(90*time.Second))).failed, "70 within the grace must pass")
	assert.True(t, run(oauthScaleEvent(71, from.Add(2*time.Minute))).failed, "71 must be rejected")
	assert.True(t, run(oauthScaleEvent(70, to.Add(authenticationComponentProxyPostTestGrace+30*time.Second))).failed, "70 past the grace must be rejected")
}

// cpNoProxyName is the exact noProxy bypass test added to the eligible set by this change.
const cpNoProxyName = cpPrefix + " oauth-server should bypass proxy by directly connecting to idp to perform OIDC login flow when spec.proxy.noProxy contains idp"

// TestAuthenticationComponentProxyScalingReplicaSet_NoProxyWindow pins the newly eligible
// noProxy bypass test through the actual registry and legacy consumer. The window behaves
// exactly like the other eligible tests: a pathological repeat (>20) whose latest occurrence
// falls inside the passing window or within the post-test grace is tolerated up to 70;
// everything else is rejected. Only a completed, Passed noProxy run grants the window, and the
// name must match exactly.
func TestAuthenticationComponentProxyScalingReplicaSet_NoProxyWindow(t *testing.T) {
	from := cpT0
	to := cpT0.Add(4 * time.Minute)

	// Legacy-consumer path: eligibility is granted only by a Passed noProxy window.
	runWith := func(win monitorapi.Interval, ev monitorapi.Interval) bool {
		events := monitorapi.Intervals{win, ev}
		evaluator := duplicateEventsEvaluator{registry: NewUniversalPathologicalEventMatchers(nil, events)}
		junits := evaluator.testDuplicatedEvents("events should not repeat", false, events, nil, false)
		name := getJUnitName("events should not repeat", "openshift-authentication")
		for _, j := range junits {
			if j.Name == name && j.FailureOutput != nil {
				return true // failed
			}
		}
		return false
	}

	passing := cpTestInterval(cpNoProxyName, "Passed", from, to)
	// A pathological repeat (24 > default 20) inside the passing noProxy window is accepted.
	assert.False(t, runWith(passing, oauthScaleEvent(24, from.Add(2*time.Minute))),
		"count 24 inside the passing noProxy window must be accepted")

	// Non-Passed noProxy runs (failed/skipped/incomplete) grant no window: 24 is then rejected.
	for _, status := range []string{"Failed", "Skipped", "DidNotFinish"} {
		win := cpTestInterval(cpNoProxyName, status, from, to)
		assert.Truef(t, runWith(win, oauthScaleEvent(24, from.Add(2*time.Minute))),
			"status %q must NOT grant the noProxy window", status)
	}

	// A near-name but unlisted test (truncated suffix) must not grant the window.
	nearName := cpPrefix + " oauth-server should bypass proxy by directly connecting to idp to perform OIDC login flow"
	assert.True(t, runWith(cpTestInterval(nearName, "Passed", from, to), oauthScaleEvent(24, from.Add(2*time.Minute))),
		"a near-name unlisted test must not grant the noProxy window")

	// Matcher-level boundary checks scoped to the passing noProxy window.
	m := scalingMatcher(t, monitorapi.Intervals{passing})
	assert.True(t, m.Allows(oauthScaleEvent(70, to.Add(90*time.Second)), v1.HighlyAvailableTopologyMode),
		"70 within the grace of the noProxy window is accepted")
	assert.False(t, m.Allows(oauthScaleEvent(70, to.Add(authenticationComponentProxyPostTestGrace+30*time.Second)), v1.HighlyAvailableTopologyMode),
		"70 past the noProxy grace is rejected")
	// cover [end+grace-0.5s, end+grace+0.5s) straddles the grace boundary -> rejected.
	assert.False(t, m.Allows(oauthScaleEvent(70, to.Add(authenticationComponentProxyPostTestGrace-500*time.Millisecond)), v1.HighlyAvailableTopologyMode),
		"one-second cover straddling the noProxy grace boundary is rejected")
	assert.False(t, m.Allows(oauthScaleEvent(71, from.Add(2*time.Minute)), v1.HighlyAvailableTopologyMode),
		"71 inside the noProxy window is rejected")
}

type junitapiCaseResult struct {
	failed bool
	output string
}

// deploymentUpdatedSeries returns one observation of the same authentication-operator
// DeploymentUpdated Event object per count in [fromCount, toCount], spaced 5s apart starting at
// start. created is the object's firstTimestamp: series sharing it are the same object.
func deploymentUpdatedSeries(created time.Time, fromCount, toCount int, start time.Time) monitorapi.Intervals {
	return eventSeries("openshift-authentication-operator", "authentication-operator", "DeploymentUpdated", created, fromCount, toCount, start, false)
}

// aggregatedScalingSeries mirrors the oauth-openshift ScalingReplicaSet Event object the recorder
// aggregates ("combined from similar events"): its message, and so its hmsg, changes on every
// update while the count keeps accumulating.
func aggregatedScalingSeries(created time.Time, fromCount, toCount int, start time.Time) monitorapi.Intervals {
	return eventSeries("openshift-authentication", "oauth-openshift", "ScalingReplicaSet", created, fromCount, toCount, start, true)
}

func eventSeries(namespace, deployment, reason string, created time.Time, fromCount, toCount int, start time.Time, varyMessage bool) monitorapi.Intervals {
	var series monitorapi.Intervals
	for count := fromCount; count <= toCount; count++ {
		at := start.Add(time.Duration(count-fromCount) * 5 * time.Second)
		observation := oauthScaleEventNS(namespace, deployment, reason, count, at)
		observation.Message.Annotations["firstTimestamp"] = created.Format(time.RFC3339)
		if varyMessage {
			observation.Message.HumanMessage = fmt.Sprintf("(combined from similar events): Scaled up replica set oauth-openshift-%d from 0 to 3", count)
			observation.Locator.Keys[monitorapi.LocatorHmsgKey] = fmt.Sprintf("%010d", count)
		}
		series = append(series, observation)
	}
	return series
}

// TestAuthenticationComponentProxy_DiscountsInWindowOccurrences pins that occurrences of an Event
// produced inside a proxy test window are not charged to a later, unrelated rollout of the same
// deployment. Kubernetes Event counts are cumulative, so without the discount the proxy test
// (here 12 occurrences, as observed in CI) leaves too little headroom under the default
// threshold of 20 for e.g. the OAuthServer IdP serial tests that run afterwards.
func TestAuthenticationComponentProxy_DiscountsInWindowOccurrences(t *testing.T) {
	from := cpT0
	to := cpT0.Add(9 * time.Minute)
	later := to.Add(30 * time.Minute) // well past the post-test grace

	withDuplicates := func(series monitorapi.Intervals) monitorapi.Intervals {
		// The monitor records each observation twice (with and without the pathological mark).
		var out monitorapi.Intervals
		for _, observation := range series {
			out = append(out, observation, observation)
		}
		return out
	}
	concat := func(parts ...monitorapi.Intervals) monitorapi.Intervals {
		var out monitorapi.Intervals
		for _, part := range parts {
			out = append(out, part...)
		}
		return out
	}

	created := from.Add(-time.Hour)
	install := deploymentUpdatedSeries(created, 1, 4, created)
	proxyTest := deploymentUpdatedSeries(created, 5, 16, from.Add(time.Minute))

	for _, tc := range []struct {
		name       string
		testStatus string
		events     monitorapi.Intervals
		namespace  string
		wantFailed bool
	}{
		{
			name:       "later rollout within threshold once proxy occurrences are discounted",
			testStatus: "Passed",
			events:     concat(install, proxyTest, deploymentUpdatedSeries(created, 17, 22, later)),
		},
		{
			name:       "duplicated observations are not double counted",
			testStatus: "Passed",
			events:     withDuplicates(concat(install, proxyTest, deploymentUpdatedSeries(created, 17, 22, later))),
		},
		{
			name:       "later rollout still over threshold after the discount",
			testStatus: "Passed",
			events:     concat(install, proxyTest, deploymentUpdatedSeries(created, 17, 33, later)),
			wantFailed: true,
		},
		{
			name:       "no discount without a passing proxy test",
			testStatus: "Failed",
			events:     concat(install, proxyTest, deploymentUpdatedSeries(created, 17, 22, later)),
			wantFailed: true,
		},
		{
			name:       "recreated Event is not discounted by the occurrences of the old one",
			testStatus: "Passed",
			events:     concat(install, proxyTest, deploymentUpdatedSeries(later, 1, 21, later)),
			wantFailed: true,
		},
		{
			name:       "batched increment entering the window is not discounted",
			testStatus: "Passed",
			// count jumps 4 -> 16 in one observation: those occurrences may predate the window.
			events: concat(install,
				deploymentUpdatedSeries(created, 16, 16, from.Add(time.Minute)),
				deploymentUpdatedSeries(created, 17, 22, later)),
			wantFailed: true,
		},
		{
			name:       "batched increment inside the window is discounted",
			testStatus: "Passed",
			// count jumps 5 -> 16 between two observations that are both inside the window.
			events: concat(install,
				deploymentUpdatedSeries(created, 5, 5, from.Add(time.Minute)),
				deploymentUpdatedSeries(created, 16, 16, from.Add(3*time.Minute)),
				deploymentUpdatedSeries(created, 17, 22, later)),
		},
		{
			name:       "aggregated Event whose message changes on every update is discounted",
			testStatus: "Passed",
			namespace:  "openshift-authentication",
			events: concat(
				aggregatedScalingSeries(from, 2, 16, from.Add(time.Minute)),
				aggregatedScalingSeries(from, 17, 30, later)),
		},
		{
			name:       "aggregated Event still over threshold after the discount",
			testStatus: "Passed",
			namespace:  "openshift-authentication",
			events: concat(
				aggregatedScalingSeries(from, 2, 16, from.Add(time.Minute)),
				aggregatedScalingSeries(from, 17, 37, later)),
			wantFailed: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := append(monitorapi.Intervals{cpTestInterval(cpNoProxyName, tc.testStatus, from, to)}, tc.events...)
			evaluator := duplicateEventsEvaluator{registry: NewUniversalPathologicalEventMatchers(nil, events)}
			junits := evaluator.testDuplicatedEvents("events should not repeat", false, events, nil, false)
			namespace := tc.namespace
			if namespace == "" {
				namespace = "openshift-authentication-operator"
			}
			name := getJUnitName("events should not repeat", namespace)
			failed := false
			for _, j := range junits {
				if j.Name == name && j.FailureOutput != nil {
					failed = true
				}
			}
			assert.Equal(t, tc.wantFailed, failed)
		})
	}
}
