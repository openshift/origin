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
// within authenticationComponentProxyPostTestGrace AFTER that test ends (no pre-test grace).
// Repeats outside those windows are allowed only if, once the occurrences of the same Event
// object that happened inside the windows are subtracted, they are within the default
// DuplicateEventThreshold. It is a bounded proximity allowance, not a causal claim that the
// tests cause the rollouts, and not a claim the events are harmless.

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
	firstTimestamp := latest.Add(-1 * time.Hour)
	coverEnd := latest.Add(time.Second)
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
					"firstTimestamp": firstTimestamp.Format(time.RFC3339),
					"lastTimestamp":  latest.Format(time.RFC3339),
				},
			},
		},
		From: latest,
		To:   coverEnd,
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

func cpEventDetails(event monitorapi.Interval, from, to time.Time) string {
	grace := authenticationComponentProxyPostTestGrace
	graceEnd := to.Add(grace)
	return fmt.Sprintf("count=%d, firstTimestamp=%s, event cover=[%s, %s), cover duration=%s, test window=[%s, %s], test duration=%s, offset from test start=%s, offset from test end=%s, post-test grace=%s, grace end=%s, offset from grace end=%s, cover end offset from grace end=%s",
		GetTimesAnEventHappened(event.Message), event.Message.Annotations["firstTimestamp"],
		event.From.Format(time.RFC3339Nano), event.To.Format(time.RFC3339Nano), event.To.Sub(event.From),
		from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano), to.Sub(from),
		event.From.Sub(from), event.From.Sub(to), grace, graceEnd.Format(time.RFC3339Nano), event.From.Sub(graceEnd), event.To.Sub(graceEnd))
}

func TestAuthenticationComponentProxyScalingReplicaSet_EligibleDuringEachTest(t *testing.T) {
	// Every one of the six names (including the T3 hot-reload no-rollout test) authorizes the
	// tolerance for an in-window repeat at the cap.
	for i, name := range cpEligibleFullNames {
		from := cpT0.Add(time.Duration(i) * 10 * time.Minute)
		to := from.Add(4 * time.Minute)
		events := monitorapi.Intervals{cpTestInterval(name, "Passed", from, to)}
		m := scalingMatcher(t, events)
		eventTime := from.Add(2 * time.Minute)
		inWindow := oauthScaleEvent(70, eventTime)
		assert.Truef(t, m.Allows(inWindow, v1.HighlyAvailableTopologyMode),
			"event during eligible test %d must be allowed: %s", i+1, cpEventDetails(inWindow, from, to))
	}
}

func TestAuthenticationComponentProxyScalingReplicaSet_PostTestGraceAndBoundaries(t *testing.T) {
	const (
		observedOffset  = 29 * time.Second
		earlyOffset     = 90 * time.Second
		laterOffset     = 150 * time.Second
		pastGraceOffset = 30 * time.Second
	)
	grace := authenticationComponentProxyPostTestGrace
	events, from, to := onePassingWindow()
	m := scalingMatcher(t, events)
	insideWindow := from.Add(2 * time.Minute)
	observedAfterEnd := to.Add(observedOffset)
	earlyAfterEnd := to.Add(earlyOffset)
	laterAfterEnd := to.Add(laterOffset)
	strictlyWithinGrace := to.Add(grace - 2*time.Second)
	coverEndsAtGrace := to.Add(grace - time.Second)
	graceEnd := to.Add(grace)
	beforeStart := from.Add(-10 * time.Second)
	pastGrace := graceEnd.Add(pastGraceOffset)

	cases := []struct {
		name   string
		latest time.Time
		allow  bool
	}{
		{"inside window", insideWindow, true},
		{"exact window end", to, true},
		{fmt.Sprintf("%s after end (observed real offset)", observedOffset), observedAfterEnd, true},
		{fmt.Sprintf("%s after end (within %s grace)", earlyOffset, grace), earlyAfterEnd, true},
		{fmt.Sprintf("%s after end (within %s grace)", laterOffset, grace), laterAfterEnd, true},
		{"cover strictly within grace", strictlyWithinGrace, true},
		{"cover ends at grace boundary", coverEndsAtGrace, false},
		{"exact grace end", graceEnd, false},
		{"before test start", beforeStart, false},
		{fmt.Sprintf("%s past %s grace", pastGraceOffset, grace), pastGrace, false},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s (latest=%s, offset from test end=%s)", c.name, c.latest.Format(time.RFC3339Nano), c.latest.Sub(to)), func(t *testing.T) {
			event := oauthScaleEvent(70, c.latest)
			t.Logf("%s", cpEventDetails(event, from, to))
			got := m.Allows(event, v1.HighlyAvailableTopologyMode)
			assert.Equalf(t, c.allow, got, "case %q: %s", c.name, cpEventDetails(event, from, to))
		})
	}
}

func TestAuthenticationComponentProxyScalingReplicaSet_SecondPrecisionGraceBoundary(t *testing.T) {
	grace := authenticationComponentProxyPostTestGrace
	events, from, to := onePassingWindow()
	m := scalingMatcher(t, events)

	// The latest occurrence's whole second-precision cover [t, t+1s) must fit within the
	// post-grace window; an occurrence whose cover extends past end+grace is NOT allowed, avoiding
	// the earlier unsafe point assumption that accepted +500ms past the end.
	wellWithin := to.Add(grace - time.Minute)
	withinEvent := oauthScaleEvent(70, wellWithin)
	assert.Truef(t, m.Allows(withinEvent, v1.HighlyAvailableTopologyMode),
		"whole event cover fits within grace: %s", cpEventDetails(withinEvent, from, to))

	// end+grace-0.5s: cover is [end+grace-0.5s, end+grace+0.5s), which extends past end+grace -> not allowed.
	straddles := to.Add(grace - 500*time.Millisecond)
	straddlingEvent := oauthScaleEvent(70, straddles)
	assert.Falsef(t, m.Allows(straddlingEvent, v1.HighlyAvailableTopologyMode),
		"cover straddling the grace boundary -> conservatively rejected: %s", cpEventDetails(straddlingEvent, from, to))
}

func TestAuthenticationComponentProxyScalingReplicaSet_OverCapRejectedEvenInWindow(t *testing.T) {
	events, from, to := onePassingWindow()
	m := scalingMatcher(t, events)
	duringTest := from.Add(2 * time.Minute)
	over := oauthScaleEvent(71, duringTest)
	assert.Falsef(t, m.Allows(over, v1.HighlyAvailableTopologyMode),
		"over-cap event must be rejected even inside an eligible window: %s", cpEventDetails(over, from, to))
	// The cap is enforced across the whole eligibility window, including the post-test grace.
	duringGrace := to.Add(90 * time.Second)
	overInGrace := oauthScaleEvent(71, duringGrace)
	assert.Falsef(t, m.Allows(overInGrace, v1.HighlyAvailableTopologyMode),
		"over-cap event must be rejected even within the post-test grace: %s", cpEventDetails(overInGrace, from, to))
}

func TestAuthenticationComponentProxyScalingReplicaSet_NegativeControls(t *testing.T) {
	events, from, to := onePassingWindow()
	m := scalingMatcher(t, events)
	in := from.Add(2 * time.Minute)
	t.Logf("negative controls: %s", cpEventDetails(oauthScaleEvent(30, in), from, to))

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
		eventTime := from.Add(2 * time.Minute)
		inWindow := oauthScaleEvent(70, eventTime)
		allowed, _ := reg.AllowedByAny(inWindow, v1.HighlyAvailableTopologyMode)
		assert.Falsef(t, allowed, "status %q must NOT grant the exception: %s", status, cpEventDetails(inWindow, from, to))
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
	gapTime := wA1.Add(11 * time.Minute)
	inGap := oauthScaleEvent(70, gapTime) // well past A+grace, before B
	assert.Falsef(t, m.Allows(inGap, v1.HighlyAvailableTopologyMode),
		"latest occurrence in the inter-test gap must not be allowed despite earlier overlap; first window: %s; second window: %s",
		cpEventDetails(inGap, wA0, wA1), cpEventDetails(inGap, wB0, wB1))
}

func TestAuthenticationComponentProxyScalingReplicaSet_DegenerateWindowExcluded(t *testing.T) {
	// Fail-closed: a test that reports Passed but has a missing or non-positive span (zero
	// timestamps, or To not strictly after From) yields no eligible window, so the matcher is
	// not even registered and grants no exception.
	zero := time.Time{}
	reversedStart := cpT0.Add(time.Minute)
	probeTime := cpT0.Add(2 * time.Minute)
	cases := []struct {
		name     string
		from, to time.Time
	}{
		{"zero from and to", zero, zero},
		{"zero to only", cpT0, zero},
		{"to equals from", cpT0, cpT0},
		{"to before from", reversedStart, cpT0},
	}
	for _, c := range cases {
		events := monitorapi.Intervals{cpTestInterval(cpEligibleFullNames[0], "Passed", c.from, c.to)}
		reg := NewUniversalPathologicalEventMatchers(nil, events)
		// Probe near cpT0 where a real window would have been.
		ev := oauthScaleEvent(70, probeTime)
		allowed, _ := reg.AllowedByAny(ev, v1.HighlyAvailableTopologyMode)
		assert.Falsef(t, allowed, "degenerate window %q must grant no exception: %s", c.name, cpEventDetails(ev, c.from, c.to))
	}
}

// The operator event allowances apply only during eligible passing tests and their
// post-test grace, with separate caps for DeploymentUpdated and OperatorStatusChanged.
func TestAuthenticationComponentProxyOperatorEvents(t *testing.T) {
	grace := authenticationComponentProxyPostTestGrace
	events, from, to := onePassingWindow()
	duringTest := from.Add(time.Minute)
	withinGrace := to.Add(grace - time.Minute)
	beforeTest := from.Add(-time.Second)
	pastGrace := to.Add(grace + 30*time.Second)
	straddlingGrace := to.Add(grace - 500*time.Millisecond)
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
				{"at cap during test", event.cap, duringTest, true},
				{"over cap during test", event.cap + 1, duringTest, false},
				{"at cap within grace", event.cap, withinGrace, true},
				{"over cap within grace", event.cap + 1, withinGrace, false},
				{"before test", event.cap, beforeTest, false},
				{"past grace", event.cap, pastGrace, false},
				{"straddling grace boundary", event.cap, straddlingGrace, false},
			} {
				t.Run(fmt.Sprintf("%s (latest=%s)", tc.name, tc.latest.Format(time.RFC3339Nano)), func(t *testing.T) {
					ev := oauthScaleEventNS("openshift-authentication-operator", "authentication-operator", event.reason, tc.count, tc.latest)
					t.Logf("cap=%d: %s", event.cap, cpEventDetails(ev, from, to))
					assert.Equalf(t, tc.allow, m.Allows(ev, v1.HighlyAvailableTopologyMode), "cap=%d: %s", event.cap, cpEventDetails(ev, from, to))
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
		t.Logf("legacy consumer: %s", cpEventDetails(ev, from, to))
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

	duringTest := from.Add(2 * time.Minute)
	assert.False(t, run(oauthScaleEvent(70, duringTest)).failed, "in-window event must pass the legacy consumer")
	duringGrace := to.Add(90 * time.Second)
	assert.Falsef(t, run(oauthScaleEvent(70, duringGrace)).failed,
		"event within the %s grace must pass", authenticationComponentProxyPostTestGrace)
	assert.True(t, run(oauthScaleEvent(71, duringTest)).failed, "over-cap event must be rejected")
	afterGrace := to.Add(authenticationComponentProxyPostTestGrace + 30*time.Second)
	assert.Truef(t, run(oauthScaleEvent(70, afterGrace)).failed,
		"event past the %s grace must be rejected", authenticationComponentProxyPostTestGrace)
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
		t.Logf("noProxy legacy consumer (status=%s): %s", win.Message.Annotations[monitorapi.AnnotationStatus], cpEventDetails(ev, win.From, win.To))
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
	duringTest := from.Add(2 * time.Minute)
	assert.False(t, runWith(passing, oauthScaleEvent(24, duringTest)),
		"event inside the passing noProxy window must be accepted")

	// Non-Passed noProxy runs (failed/skipped/incomplete) grant no window: 24 is then rejected.
	for _, status := range []string{"Failed", "Skipped", "DidNotFinish"} {
		win := cpTestInterval(cpNoProxyName, status, from, to)
		assert.Truef(t, runWith(win, oauthScaleEvent(24, duringTest)),
			"status %q must NOT grant the noProxy window", status)
	}

	// A near-name but unlisted test (truncated suffix) must not grant the window.
	nearName := cpPrefix + " oauth-server should bypass proxy by directly connecting to idp to perform OIDC login flow"
	assert.True(t, runWith(cpTestInterval(nearName, "Passed", from, to), oauthScaleEvent(24, duringTest)),
		"a near-name unlisted test must not grant the noProxy window")

	// Matcher-level boundary checks scoped to the passing noProxy window.
	m := scalingMatcher(t, monitorapi.Intervals{passing})
	duringGrace := to.Add(90 * time.Second)
	withinGrace := oauthScaleEvent(70, duringGrace)
	assert.Truef(t, m.Allows(withinGrace, v1.HighlyAvailableTopologyMode),
		"event within the grace of the noProxy window is accepted: %s", cpEventDetails(withinGrace, from, to))
	afterGrace := to.Add(authenticationComponentProxyPostTestGrace + 30*time.Second)
	pastGrace := oauthScaleEvent(70, afterGrace)
	assert.Falsef(t, m.Allows(pastGrace, v1.HighlyAvailableTopologyMode),
		"event past the noProxy grace is rejected: %s", cpEventDetails(pastGrace, from, to))
	// cover [end+grace-0.5s, end+grace+0.5s) straddles the grace boundary -> rejected.
	straddlingGrace := to.Add(authenticationComponentProxyPostTestGrace - 500*time.Millisecond)
	straddles := oauthScaleEvent(70, straddlingGrace)
	assert.Falsef(t, m.Allows(straddles, v1.HighlyAvailableTopologyMode),
		"event cover straddling the noProxy grace boundary is rejected: %s", cpEventDetails(straddles, from, to))
	overCap := oauthScaleEvent(71, duringTest)
	assert.Falsef(t, m.Allows(overCap, v1.HighlyAvailableTopologyMode),
		"over-cap event inside the noProxy window is rejected: %s", cpEventDetails(overCap, from, to))
}

type junitapiCaseResult struct {
	failed bool
	output string
}

// This is a synthetic counter replay using the identity ambiguity reported in review:
// independent Event objects can share every field of the former fallback key. It does not
// claim that the reviewed CI artifact actually borrowed credits.
func TestAuthenticationComponentProxy_CollidingEventIdentities(t *testing.T) {
	from := cpT0
	to := from.Add(9 * time.Minute)
	for _, tc := range []struct {
		name           string
		insideUID      string
		outsideUID     string
		emptyUID       bool
		wantDiscounted bool
	}{
		{name: "same UID retains cumulative discount", insideUID: "event-a", outsideUID: "event-a", wantDiscounted: true},
		{name: "distinct UIDs cannot share credits", insideUID: "event-a", outsideUID: "event-b"},
		{name: "missing UIDs cannot share credits"},
		{name: "empty UIDs cannot share credits", emptyUID: true},
		{name: "missing UID cannot consume identified credits", insideUID: "event-a"},
		{name: "missing UID cannot contribute identified credits", outsideUID: "event-b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			created := from.Add(-time.Hour)
			insideTime := from.Add(time.Minute)
			inside := deploymentUpdatedObject(tc.insideUID, created).series(1, 12, insideTime)
			outsideTime := to.Add(30 * time.Minute)
			outside := deploymentUpdatedObject(tc.outsideUID, created).series(21, 21, outsideTime)[0]
			t.Logf("in-window observations (created=%s): first: %s; last: %s", created.Format(time.RFC3339Nano),
				cpEventDetails(inside[0], from, to), cpEventDetails(inside[len(inside)-1], from, to))
			t.Logf("outside observation: %s", cpEventDetails(outside, from, to))
			if tc.emptyUID {
				for _, i := range inside {
					i.Message.Annotations[monitorapi.AnnotationEventUID] = ""
				}
				outside.Message.Annotations[monitorapi.AnnotationEventUID] = ""
			}

			oldFallbackKey := func(i monitorapi.Interval) string {
				return fmt.Sprintf("%s - reason/%s firstTimestamp/%s %s", i.Locator.OldLocator(),
					i.Message.Reason, i.Message.Annotations["firstTimestamp"], i.Message.HumanMessage)
			}
			require.Equal(t, oldFallbackKey(inside[0]), oldFallbackKey(outside),
				"the observations must collide under the former fallback identity")

			events := append(monitorapi.Intervals{cpTestInterval(cpNoProxyName, "Passed", from, to)}, inside...)
			events = append(events, outside)
			registry := NewUniversalPathologicalEventMatchers(nil, events)
			matcher, err := registry.GetMatcherByName("AuthenticationComponentProxyDeploymentUpdated")
			require.NoError(t, err)
			replay := matcher.(*OverlapOtherIntervalsPathologicalEventMatcher)
			wantCredits := 0
			if tc.wantDiscounted {
				wantCredits = 12
			}
			assert.Equalf(t, wantCredits, replay.inWindowOccurrences[newEventObservation(outside)],
				"only observations of the same identified Event may supply credits (in-window count=%d)", GetTimesAnEventHappened(inside[len(inside)-1].Message))
			for observation := range replay.inWindowOccurrences {
				assert.True(t, observation.event.valid, "UID-less observations must not enter the credit map")
			}
			assert.Equalf(t, tc.wantDiscounted, matcher.Allows(outside, v1.HighlyAvailableTopologyMode),
				"outside count=%d must be rejected unless the same UID earned the in-window count=%d credits",
				GetTimesAnEventHappened(outside.Message), GetTimesAnEventHappened(inside[len(inside)-1].Message))

			evaluator := duplicateEventsEvaluator{registry: registry}
			junits := evaluator.testDuplicatedEvents("events should not repeat", false, events, nil, false)
			found := false
			for _, junit := range junits {
				if junit.Name == getJUnitName("events should not repeat", "openshift-authentication-operator") {
					found = true
					assert.Equal(t, !tc.wantDiscounted, junit.FailureOutput != nil,
						"the JUnit result must preserve the outside-window failure when identity is unavailable or different")
				}
			}
			require.True(t, found, "expected the authentication-operator JUnit result")
		})
	}
}

func TestAuthenticationComponentProxy_UIDLessInWindowCap(t *testing.T) {
	for _, tc := range []struct {
		name     string
		emptyUID bool
	}{
		{name: "missing UID"},
		{name: "empty UID", emptyUID: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events, from, to := onePassingWindow()
			atCapTime := from.Add(time.Minute)
			atCap := operatorStatusChangedObject("", from).series(30, 30, atCapTime)[0]
			overCapTime := from.Add(2 * time.Minute)
			overCap := operatorStatusChangedObject("", from).series(31, 31, overCapTime)[0]
			t.Logf("at cap: %s; over cap: %s", cpEventDetails(atCap, from, to), cpEventDetails(overCap, from, to))
			if tc.emptyUID {
				atCap.Message.Annotations[monitorapi.AnnotationEventUID] = ""
				overCap.Message.Annotations[monitorapi.AnnotationEventUID] = ""
			}
			events = append(events, atCap, overCap)
			registry := NewUniversalPathologicalEventMatchers(nil, events)
			matcher, err := registry.GetMatcherByName("AuthenticationComponentProxyOperatorStatusChanged")
			require.NoError(t, err)
			assert.Empty(t, matcher.(*OverlapOtherIntervalsPathologicalEventMatcher).inWindowOccurrences,
				"UID-less observations must not contribute cumulative credits")
			assert.True(t, matcher.Allows(atCap, v1.HighlyAvailableTopologyMode),
				"the direct in-window allowance must still apply without a UID")
			assert.False(t, matcher.Allows(overCap, v1.HighlyAvailableTopologyMode),
				"the direct in-window allowance must still enforce its cap")
		})
	}
}

// testEventObject is a Kubernetes Event object whose observations the monitor records.
type testEventObject struct {
	namespace, deployment, reason string
	// uid is the Event object's UID; empty mimics intervals recorded without it.
	uid string
	// created is the object's firstTimestamp.
	created time.Time
	// message overrides the default human message.
	message string
	// aggregated mirrors an object the recorder aggregates ("combined from similar events"):
	// its message, and so its hmsg, changes on every update while the count keeps accumulating.
	aggregated bool
}

func deploymentUpdatedObject(uid string, created time.Time) testEventObject {
	return testEventObject{namespace: "openshift-authentication-operator", deployment: "authentication-operator",
		reason: "DeploymentUpdated", uid: uid, created: created}
}

func operatorStatusChangedObject(uid string, created time.Time) testEventObject {
	return testEventObject{namespace: "openshift-authentication-operator", deployment: "authentication-operator",
		reason: "OperatorStatusChanged", uid: uid, created: created}
}

func aggregatedScalingObject(uid string, created time.Time) testEventObject {
	return testEventObject{namespace: "openshift-authentication", deployment: "oauth-openshift",
		reason: "ScalingReplicaSet", uid: uid, created: created, aggregated: true}
}

// series returns one observation of the object per count in [fromCount, toCount], spaced 5s
// apart starting at start.
func (o testEventObject) series(fromCount, toCount int, start time.Time) monitorapi.Intervals {
	var series monitorapi.Intervals
	for count := fromCount; count <= toCount; count++ {
		at := start.Add(time.Duration(count-fromCount) * 5 * time.Second)
		observation := oauthScaleEventNS(o.namespace, o.deployment, o.reason, count, at)
		observation.Message.Annotations["firstTimestamp"] = o.created.Format(time.RFC3339)
		if o.uid != "" {
			observation.Message.Annotations[monitorapi.AnnotationEventUID] = o.uid
		}
		if o.message != "" {
			observation.Message.HumanMessage = o.message
		}
		if o.aggregated {
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
	du := deploymentUpdatedObject("du", created)
	install := du.series(1, 4, created)
	proxyStart := from.Add(time.Minute)
	proxyLater := from.Add(3 * time.Minute)
	independentStart := created.Add(time.Second)
	proxyTest := du.series(5, 16, proxyStart)

	// independent shares resource, reason and firstTimestamp with du but is another object.
	independent := deploymentUpdatedObject("du-independent", created)
	independent.message = "Updated Deployment.apps/oauth-openshift -n openshift-authentication because it changed (independent)"
	independentWithoutUIDs := independent
	independentWithoutUIDs.uid = ""
	duWithoutUIDs := du
	duWithoutUIDs.uid = ""

	osc := operatorStatusChangedObject("osc", created)
	scaling := aggregatedScalingObject("scaling", from)
	scalingWithoutUIDs := scaling
	scalingWithoutUIDs.uid = ""

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
			events:     concat(install, proxyTest, du.series(17, 22, later)),
		},
		{
			name:       "duplicated observations are not double counted",
			testStatus: "Passed",
			events:     withDuplicates(concat(install, proxyTest, du.series(17, 22, later))),
		},
		{
			name:       "later rollout still over threshold after the discount",
			testStatus: "Passed",
			events:     concat(install, proxyTest, du.series(17, 33, later)),
			wantFailed: true,
		},
		{
			name:       "no discount without a passing proxy test",
			testStatus: "Failed",
			events:     concat(install, proxyTest, du.series(17, 22, later)),
			wantFailed: true,
		},
		{
			name:       "recreated Event is not discounted by the occurrences of the old one",
			testStatus: "Passed",
			events:     concat(install, proxyTest, deploymentUpdatedObject("du-recreated", later).series(1, 21, later)),
			wantFailed: true,
		},
		{
			name:       "batched increment entering the window is not discounted",
			testStatus: "Passed",
			// count jumps 4 -> 16 in one observation: those occurrences may predate the window.
			events: concat(install,
				du.series(16, 16, proxyStart),
				du.series(17, 22, later)),
			wantFailed: true,
		},
		{
			name:       "batched increment inside the window is discounted",
			testStatus: "Passed",
			// count jumps 5 -> 16 between two observations that are both inside the window.
			events: concat(install,
				du.series(5, 5, proxyStart),
				du.series(16, 16, proxyLater),
				du.series(17, 22, later)),
		},
		{
			name:       "aggregated Event whose message changes on every update is discounted",
			testStatus: "Passed",
			namespace:  "openshift-authentication",
			events: concat(
				scaling.series(2, 16, proxyStart),
				scaling.series(17, 30, later)),
		},
		{
			name:       "aggregated Event still over threshold after the discount",
			testStatus: "Passed",
			namespace:  "openshift-authentication",
			events: concat(
				scaling.series(2, 16, proxyStart),
				scaling.series(17, 37, later)),
			wantFailed: true,
		},
		{
			name:       "aggregated Event recorded without UID is conservatively not discounted",
			testStatus: "Passed",
			namespace:  "openshift-authentication",
			events: concat(
				scalingWithoutUIDs.series(2, 16, proxyStart),
				scalingWithoutUIDs.series(17, 30, later)),
			wantFailed: true,
		},
		{
			name:       "independent Event sharing resource, reason and firstTimestamp inherits no discount",
			testStatus: "Passed",
			// du's in-window occurrences must not be credited to independent, whose 21 occurrences
			// all happened outside the window.
			events: concat(
				independent.series(1, 20, independentStart),
				du.series(1, 12, proxyStart),
				independent.series(21, 21, later)),
			wantFailed: true,
		},
		{
			name:       "independent Event recorded without UIDs inherits no discount",
			testStatus: "Passed",
			events: concat(
				independentWithoutUIDs.series(1, 20, independentStart),
				duWithoutUIDs.series(1, 12, proxyStart),
				independentWithoutUIDs.series(21, 21, later)),
			wantFailed: true,
		},
		{
			name:       "discount applies even when the cumulative count exceeds the in-window cap",
			testStatus: "Passed",
			// 21 in-window occurrences (cap 30) plus 10 outside: cumulative 31 > 30.
			events: concat(
				osc.series(1, 21, proxyStart),
				osc.series(22, 31, later)),
		},
		{
			name:       "aggregated Event discounted even when the cumulative count exceeds the in-window cap",
			testStatus: "Passed",
			namespace:  "openshift-authentication",
			// 60 in-window occurrences (cap 70) plus 11 outside: cumulative 71 > 70.
			events: concat(
				scaling.series(1, 60, proxyStart),
				scaling.series(61, 71, later)),
		},
		{
			name:       "out-of-window occurrences over threshold rejected above the in-window cap",
			testStatus: "Passed",
			// 21 in-window occurrences plus 21 outside.
			events: concat(
				osc.series(1, 21, proxyStart),
				osc.series(22, 42, later)),
			wantFailed: true,
		},
		{
			name:       "in-window cap still enforced",
			testStatus: "Passed",
			// 31 in-window occurrences exceed the OperatorStatusChanged cap of 30.
			events:     osc.series(1, 31, proxyStart),
			wantFailed: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := append(monitorapi.Intervals{cpTestInterval(cpNoProxyName, tc.testStatus, from, to)}, tc.events...)
			for _, event := range tc.events {
				t.Logf("%s", cpEventDetails(event, from, to))
			}
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
