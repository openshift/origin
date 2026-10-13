package pathologicaleventlibrary

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// All timestamps are synthetic and sanitized. The window boundaries mirror the shape of a
// real AuthenticationComponentProxy serial run (five passing tests with gaps between them)
// without copying any run-specific identifiers.
var (
	testBase = time.Date(2026, 1, 2, 20, 0, 0, 0, time.UTC)

	winT1 = EligibleWindow{Label: "T1", IncrementsExpected: true, From: testBase.Add(10 * time.Minute), To: testBase.Add(14 * time.Minute)}
	winT3 = EligibleWindow{Label: "T3", IncrementsExpected: false, From: testBase.Add(56 * time.Minute), To: testBase.Add(61 * time.Minute)}
	winT4 = EligibleWindow{Label: "T4", IncrementsExpected: true, From: testBase.Add(62 * time.Minute), To: testBase.Add(67 * time.Minute)}

	rolloutWindows = []EligibleWindow{winT1, winT3, winT4}
)

// obs builds a generic CounterObservation. id is the identity (empty => no stable identity);
// seq < 0 means "no sequence". A domain Note is attached to mirror how the oauth adapter
// annotates observations.
func obs(id string, seq int64, count int, observed time.Time) CounterObservation {
	o := CounterObservation{
		Identity: id,
		Count:    count,
		Observed: observed,
		Note:     "rs=" + oauthScaleDeployment + "-788ffcfbc6 scaled up 0->3 replicas",
	}
	if seq >= 0 {
		o.Sequence = seq
		o.HasSequence = true
	}
	return o
}

func classCounts(findings []AccountingFinding) map[AccountingClass]int {
	m := map[AccountingClass]int{}
	for _, f := range findings {
		m[f.Class]++
	}
	return m
}

func hasClass(findings []AccountingFinding, c AccountingClass) bool {
	return classCounts(findings)[c] > 0
}

func TestAccountIdentityGroup(t *testing.T) {
	tests := []struct {
		name            string
		uid             string
		observations    []CounterObservation
		windows         []EligibleWindow
		wantClass       AccountingClass // the primary class expected to be present
		wantIncrement   int             // increment on the first finding of wantClass (0 = skip check)
		wantWindowLabel string          // window label on the first finding of wantClass ("" = skip)
		assertAbsent    []AccountingClass
	}{
		{
			name: "eligible contiguous increments are attributed to the containing window",
			uid:  "u1",
			observations: []CounterObservation{
				obs("u1", 1, 10, winT1.From.Add(30*time.Second)),
				obs("u1", 2, 13, winT1.From.Add(60*time.Second)),
			},
			windows:         rolloutWindows,
			wantClass:       ClassAttributed,
			wantIncrement:   3,
			wantWindowLabel: "T1",
			assertAbsent:    []AccountingClass{ClassUnresolvedSpanCrosses, ClassUnresolvedNoBaseline},
		},
		{
			name: "span exactly equal to window boundaries is contained (equality)",
			uid:  "u1",
			observations: []CounterObservation{
				obs("u1", 1, 5, winT1.From),
				obs("u1", 2, 8, winT1.To),
			},
			windows:         rolloutWindows,
			wantClass:       ClassAttributed,
			wantIncrement:   3,
			wantWindowLabel: "T1",
		},
		{
			name: "post-test 45s control: span leaves the window into the gap is unresolved",
			uid:  "u1",
			observations: []CounterObservation{
				obs("u1", 1, 5, winT1.To),
				obs("u1", 2, 7, winT1.To.Add(45*time.Second)),
			},
			windows:      rolloutWindows,
			wantClass:    ClassUnresolvedSpanCrosses,
			assertAbsent: []AccountingClass{ClassAttributed},
		},
		{
			name: "batch crossing test/gap boundaries is unresolved",
			uid:  "u1",
			observations: []CounterObservation{
				obs("u1", 1, 5, winT1.From.Add(time.Minute)),
				obs("u1", 2, 20, winT4.From.Add(time.Minute)),
			},
			windows:      rolloutWindows,
			wantClass:    ClassUnresolvedSpanCrosses,
			assertAbsent: []AccountingClass{ClassAttributed},
		},
		{
			name: "span entirely outside every eligible window",
			uid:  "u1",
			observations: []CounterObservation{
				obs("u1", 1, 5, winT1.To.Add(5*time.Minute)),
				obs("u1", 2, 9, winT1.To.Add(6*time.Minute)),
			},
			windows:      rolloutWindows,
			wantClass:    ClassUnresolvedNoWindow,
			assertAbsent: []AccountingClass{ClassAttributed},
		},
		{
			name: "protected no-rollout phase (T3) is never attributed",
			uid:  "u1",
			observations: []CounterObservation{
				obs("u1", 1, 2, winT3.From.Add(10*time.Second)),
				obs("u1", 2, 5, winT3.From.Add(40*time.Second)),
			},
			windows:         rolloutWindows,
			wantClass:       ClassProtectedWindow,
			wantIncrement:   3,
			wantWindowLabel: "T3",
			assertAbsent:    []AccountingClass{ClassAttributed},
		},
		{
			name: "counter reset restarts baseline and is unresolved",
			uid:  "u1",
			observations: []CounterObservation{
				obs("u1", 1, 10, winT1.From.Add(30*time.Second)),
				obs("u1", 2, 3, winT1.From.Add(60*time.Second)),
			},
			windows:      rolloutWindows,
			wantClass:    ClassUnresolvedMalformed,
			assertAbsent: []AccountingClass{ClassAttributed},
		},
		{
			name: "out-of-order observation time across a forward step is unresolved",
			uid:  "u1",
			observations: []CounterObservation{
				obs("u1", 1, 5, winT1.From.Add(60*time.Second)),
				obs("u1", 2, 8, winT1.From.Add(30*time.Second)),
			},
			windows:      rolloutWindows,
			wantClass:    ClassUnresolvedMalformed,
			assertAbsent: []AccountingClass{ClassAttributed},
		},
		{
			name: "duplicate (uid,resourceVersion) rows are deduped, remaining step attributed",
			uid:  "u1",
			observations: []CounterObservation{
				obs("u1", 1, 5, winT1.From.Add(30*time.Second)),
				obs("u1", 1, 5, winT1.From.Add(31*time.Second)),
				obs("u1", 2, 7, winT1.From.Add(60*time.Second)),
			},
			windows:         rolloutWindows,
			wantClass:       ClassAttributed,
			wantIncrement:   2,
			wantWindowLabel: "T1",
			assertAbsent:    []AccountingClass{ClassUnresolvedMalformed},
		},
		{
			name: "collision: same resourceVersion with different counts is malformed",
			uid:  "u1",
			observations: []CounterObservation{
				obs("u1", 1, 5, winT1.From.Add(30*time.Second)),
				obs("u1", 1, 9, winT1.From.Add(31*time.Second)),
			},
			windows:   rolloutWindows,
			wantClass: ClassUnresolvedMalformed,
		},
		{
			name: "single observation for an identity has no reliable baseline",
			uid:  "u1",
			observations: []CounterObservation{
				obs("u1", 1, 31, winT4.From.Add(time.Minute)),
			},
			windows:      rolloutWindows,
			wantClass:    ClassUnresolvedNoBaseline,
			assertAbsent: []AccountingClass{ClassAttributed},
		},
		{
			name: "sub-second observation just after window end is not contained (strict)",
			uid:  "u1",
			observations: []CounterObservation{
				obs("u1", 1, 5, winT1.From),
				obs("u1", 2, 8, winT1.To.Add(500*time.Millisecond)),
			},
			windows:      rolloutWindows,
			wantClass:    ClassUnresolvedSpanCrosses,
			assertAbsent: []AccountingClass{ClassAttributed},
		},
		{
			name: "missing observation timestamps cannot bound the increment span",
			uid:  "u1",
			observations: []CounterObservation{
				obs("u1", 1, 5, time.Time{}),
				obs("u1", 2, 8, time.Time{}),
			},
			windows:      rolloutWindows,
			wantClass:    ClassUnresolvedNoBaseline,
			assertAbsent: []AccountingClass{ClassAttributed},
		},
		{
			name: "no stable identity (empty UID) is unresolved",
			uid:  "",
			observations: []CounterObservation{
				obs("", -1, 31, winT4.From.Add(time.Minute)),
			},
			windows:      rolloutWindows,
			wantClass:    ClassUnresolvedNoIdentity,
			assertAbsent: []AccountingClass{ClassAttributed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := accountIdentityGroup(tt.uid, tt.observations, tt.windows)
			require.True(t, hasClass(findings, tt.wantClass),
				"expected a %s finding, got: %+v", tt.wantClass, findings)
			if tt.wantIncrement != 0 || tt.wantWindowLabel != "" {
				for _, f := range findings {
					if f.Class == tt.wantClass {
						if tt.wantIncrement != 0 {
							assert.Equal(t, tt.wantIncrement, f.Increment)
						}
						if tt.wantWindowLabel != "" {
							assert.Equal(t, tt.wantWindowLabel, f.Window)
						}
						break
					}
				}
			}
			for _, absent := range tt.assertAbsent {
				assert.False(t, hasClass(findings, absent), "did not expect class %s in %+v", absent, findings)
			}
		})
	}
}

// TestAccountIdentityGroup_LateObservationRetainsBaseline is the regression for review
// finding A (discussion_r4261872203). A late/out-of-order record must NOT become the baseline
// for the following step. Sequence (sorted by resourceVersion): a T1 baseline (count 5), a
// late arrival observed earlier (count 6), then a later T1 record (count 8), then a T4 record
// (count 11). With the bug the late record (count 6) became the baseline and the T1 step was
// mis-measured as 6->8 = 2 with a span starting too early; after the fix the baseline is
// retained so the step is the correct 5->8 = 3, fully contained in T1, and the T1->T4 step
// correctly crosses windows.
func TestAccountIdentityGroup_LateObservationRetainsBaseline(t *testing.T) {
	observations := []CounterObservation{
		obs("u1", 1, 5, winT1.From.Add(30*time.Second)),  // baseline, in T1
		obs("u1", 2, 6, winT1.From.Add(10*time.Second)),  // LATE: observed before the baseline
		obs("u1", 3, 8, winT1.From.Add(90*time.Second)),  // later, in T1
		obs("u1", 4, 11, winT4.From.Add(60*time.Second)), // in T4
	}

	findings := accountIdentityGroup("u1", observations, rolloutWindows)

	require.True(t, hasClass(findings, ClassUnresolvedMalformed), "late record must be flagged malformed: %+v", findings)
	require.True(t, hasClass(findings, ClassAttributed), "retained baseline must yield an attributed step: %+v", findings)
	require.True(t, hasClass(findings, ClassUnresolvedSpanCrosses), "the T1->T4 step must cross windows: %+v", findings)

	var attributed []AccountingFinding
	for _, f := range findings {
		if f.Class == ClassAttributed {
			attributed = append(attributed, f)
		}
	}
	require.Len(t, attributed, 1)
	assert.Equal(t, 3, attributed[0].Increment, "baseline retained -> correct delta 5->8=3 (not the poisoned 6->8=2)")
	assert.Equal(t, "T1", attributed[0].Window)
	assert.Equal(t, winT1.From.Add(30*time.Second), attributed[0].SpanFrom, "span must start at the retained baseline, not the late record")
	// Guard against the pre-fix poisoned delta.
	for _, f := range attributed {
		assert.NotEqual(t, 2, f.Increment, "increment 2 would mean the late record poisoned the baseline")
	}
}

// TestAccountIdentityGroup_LateLowerCountIsNotReset proves the late-before-reset precedence
// for review finding A: a late/out-of-order observation whose count is LOWER than the current
// baseline must be classified as an out-of-order late arrival (baseline retained), NOT as a
// counter reset (which would advance the baseline to the stale, earlier record). A following
// in-window record must then attribute against the retained baseline with the correct delta.
func TestAccountIdentityGroup_LateLowerCountIsNotReset(t *testing.T) {
	observations := []CounterObservation{
		obs("u1", 1, 7, winT1.From.Add(40*time.Second)),  // baseline, count 7
		obs("u1", 2, 4, winT1.From.Add(10*time.Second)),  // LATE and LOWER count than baseline
		obs("u1", 3, 10, winT1.From.Add(80*time.Second)), // later, in T1
	}

	findings := accountIdentityGroup("u1", observations, rolloutWindows)

	var malformed, attributed []AccountingFinding
	for _, f := range findings {
		switch f.Class {
		case ClassUnresolvedMalformed:
			malformed = append(malformed, f)
		case ClassAttributed:
			attributed = append(attributed, f)
		}
	}
	require.Len(t, malformed, 1, "the late lower-count record must produce exactly one malformed finding: %+v", findings)
	assert.Contains(t, malformed[0].Detail, "out-of-order", "must be classified as late/out-of-order, not a counter reset")
	assert.NotContains(t, malformed[0].Detail, "counter reset", "a late lower-count record must NOT be treated as a reset")

	require.Len(t, attributed, 1, "retained baseline must yield one attributed step: %+v", findings)
	assert.Equal(t, 3, attributed[0].Increment, "baseline 7 retained -> 7->10 = 3 (not a reset-restarted baseline of 4)")
	assert.Equal(t, "T1", attributed[0].Window)
	assert.Equal(t, winT1.From.Add(40*time.Second), attributed[0].SpanFrom, "span must start at the retained baseline")
}

// --- end-to-end intervals helpers -------------------------------------------------------

func e2eInterval(fullName, status string, from, to time.Time) monitorapi.Interval {
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

type kubeEventOpts struct {
	namespace  string
	deployment string
	reason     string
	source     monitorapi.IntervalSource
	message    string
	count      int
	uid        string
	rv         string
	first      time.Time
	last       time.Time
	observed   time.Time
	seriesCnt  string
}

func kubeEventInterval(o kubeEventOpts) monitorapi.Interval {
	ann := map[monitorapi.AnnotationKey]string{}
	if o.count > 0 {
		ann[monitorapi.AnnotationCount] = fmt.Sprintf("%d", o.count)
	}
	if o.uid != "" {
		ann[AnnotationEventUID] = o.uid
	}
	if o.rv != "" {
		ann[AnnotationResourceVersion] = o.rv
	}
	if !o.first.IsZero() {
		ann[annotationFirstTimestamp] = o.first.Format(time.RFC3339)
	}
	if !o.last.IsZero() {
		ann[annotationLastTimestamp] = o.last.Format(time.RFC3339)
	}
	if !o.observed.IsZero() {
		ann[AnnotationSeriesLastObserved] = o.observed.Format(time.RFC3339)
	}
	if o.seriesCnt != "" {
		ann[AnnotationSeriesCount] = o.seriesCnt
	}
	src := o.source
	if src == "" {
		src = monitorapi.SourceKubeEvent
	}
	return monitorapi.Interval{
		Source: src,
		Condition: monitorapi.Condition{
			Locator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeKind,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorNamespaceKey:  o.namespace,
					monitorapi.LocatorDeploymentKey: o.deployment,
				},
			},
			Message: monitorapi.Message{
				Reason:       monitorapi.IntervalReason(o.reason),
				HumanMessage: o.message,
				Annotations:  ann,
			},
		},
		From: o.last,
		To:   o.last,
	}
}

func fullT(suffix string) string { return componentProxyTestPrefix + " " + suffix }

func TestCollectEligibleWindows(t *testing.T) {
	intervals := monitorapi.Intervals{
		// All five, passing -> eligible. Includes T2 whose name is a strict prefix of T1.
		e2eInterval(fullT(suffixT1TrustedCA), "Passed", testBase.Add(10*time.Minute), testBase.Add(14*time.Minute)),
		e2eInterval(fullT(suffixT2ValidateOIDC), "Passed", testBase.Add(43*time.Minute), testBase.Add(46*time.Minute)),
		e2eInterval(fullT(suffixT3HotReloadCA), "Passed", testBase.Add(56*time.Minute), testBase.Add(61*time.Minute)),
		e2eInterval(fullT(suffixT4FullLoginFlow), "Passed", testBase.Add(62*time.Minute), testBase.Add(67*time.Minute)),
		e2eInterval(fullT(suffixT5FallBack), "Passed", testBase.Add(73*time.Minute), testBase.Add(79*time.Minute)),
		// Not eligible: failed, flaked, did-not-finish, skipped.
		e2eInterval(fullT(suffixT1TrustedCA), "Failed", testBase, testBase.Add(time.Minute)),
		e2eInterval(fullT(suffixT4FullLoginFlow), "Flaked", testBase, testBase.Add(time.Minute)),
		e2eInterval(fullT(suffixT5FallBack), "DidNotFinish", testBase, testBase.Add(time.Minute)),
		// Not eligible: unrelated test, and a near-miss substring that is not a full match.
		e2eInterval("[sig-arch] some other test", "Passed", testBase, testBase.Add(time.Minute)),
		e2eInterval(componentProxyTestPrefix+" operator should validate OIDC IdP through component proxy with extra", "Passed", testBase, testBase.Add(time.Minute)),
	}

	windows := oauthEligibleWindows(intervals)
	require.Len(t, windows, 5)

	labels := map[string]bool{}
	for _, w := range windows {
		labels[w.Label] = true
	}
	for _, want := range []string{"T1", "T2", "T3", "T4", "T5"} {
		assert.True(t, labels[want], "missing window %s", want)
	}
	// T3 must be a protected, no-rollout window.
	for _, w := range windows {
		if w.Label == "T3" {
			assert.False(t, w.IncrementsExpected, "T3 must be a no-rollout window")
		} else {
			assert.True(t, w.IncrementsExpected, "%s should be rollout-expected", w.Label)
		}
	}
}

func TestIsCandidateOAuthScaleEvent_IgnoresWrongResourceAndSource(t *testing.T) {
	good := kubeEventOpts{namespace: oauthScaleNamespace, deployment: oauthScaleDeployment, reason: oauthScaleReason, last: testBase}
	tests := []struct {
		name string
		o    kubeEventOpts
		want bool
	}{
		{"candidate", good, true},
		{"wrong source E2E", func() kubeEventOpts { c := good; c.source = monitorapi.SourceE2ETest; return c }(), false},
		{"wrong reason", func() kubeEventOpts { c := good; c.reason = "DeploymentUpdated"; return c }(), false},
		{"wrong namespace", func() kubeEventOpts { c := good; c.namespace = "openshift-authentication-operator"; return c }(), false},
		{"wrong deployment", func() kubeEventOpts { c := good; c.deployment = "authentication-operator"; return c }(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isCandidateOAuthScaleEvent(kubeEventInterval(tt.o)))
		})
	}
}

func TestParseObservation_MessageAndOwner(t *testing.T) {
	tests := []struct {
		name          string
		message       string
		wantParseOK   bool
		wantOwner     bool
		wantFrom      int
		wantTo        int
		wantDirection string
	}{
		{"plain scaled up", "Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3", true, true, 0, 3, "up"},
		{"combined prefix scaled up", "(combined from similar events): Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3", true, true, 0, 3, "up"},
		{"scaled down", "Scaled down replica set oauth-openshift-abc123 from 3 to 1", true, true, 3, 1, "down"},
		{"wrong deployment rs not owner", "Scaled up replica set other-deploy-abc from 0 to 3", false, false, 0, 0, ""},
		{"unanchored trailing text", "Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3 and then some", false, false, 0, 0, ""},
		{"non matching message", "Something entirely different", false, false, 0, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOAuthScaleMessage(tt.message)
			assert.Equal(t, tt.wantParseOK, got.ok)
			assert.Equal(t, tt.wantOwner, got.ownerVerified)
			if tt.wantParseOK {
				assert.Equal(t, tt.wantFrom, got.fromReplicas)
				assert.Equal(t, tt.wantTo, got.toReplicas)
				assert.Equal(t, tt.wantDirection, got.direction)
			}
		})
	}
}

func TestEvaluateShadow_NeverFailsAndNeverApprovesWithoutBudget(t *testing.T) {
	// Two observations of the same event UID, both inside T4 -> attributable, but with no
	// established budget there must be no shadow approval and, crucially, no failure.
	scaleMsg := "(combined from similar events): Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3"
	intervals := monitorapi.Intervals{
		e2eInterval(fullT(suffixT4FullLoginFlow), "Passed", testBase.Add(62*time.Minute), testBase.Add(67*time.Minute)),
		kubeEventInterval(kubeEventOpts{
			namespace: oauthScaleNamespace, deployment: oauthScaleDeployment, reason: oauthScaleReason,
			message: scaleMsg, count: 5, uid: "uid-a", rv: "100",
			first: testBase.Add(63 * time.Minute), last: testBase.Add(63 * time.Minute), observed: testBase.Add(63 * time.Minute),
		}),
		kubeEventInterval(kubeEventOpts{
			namespace: oauthScaleNamespace, deployment: oauthScaleDeployment, reason: oauthScaleReason,
			message: scaleMsg, count: 8, uid: "uid-a", rv: "101",
			first: testBase.Add(63 * time.Minute), last: testBase.Add(64 * time.Minute), observed: testBase.Add(64 * time.Minute),
		}),
		// A non-candidate kube event with the same reason in a different namespace: ignored.
		kubeEventInterval(kubeEventOpts{
			namespace: "openshift-apiserver", deployment: "apiserver", reason: oauthScaleReason,
			message: "Scaled up replica set apiserver-abc from 0 to 3", count: 30, last: testBase.Add(63 * time.Minute),
		}),
	}

	report, junits := EvaluateOAuthBoundedAccountingShadow(intervals, nil)
	require.Len(t, junits, 1)
	// Always-passing: informational JUnit must never carry a failure.
	assert.Nil(t, junits[0].FailureOutput, "shadow JUnit must never fail the run")
	assert.Equal(t, oauthShadowAccountingTestName, junits[0].Name)

	assert.Equal(t, 1, report.EligibleWindows)
	assert.Equal(t, 2, report.CandidateEvents)
	assert.Equal(t, 1, report.AttributedUnits, "the 5->8 step is attributable to T4")
	assert.Equal(t, 3, report.PerWindowAttributed["T4"])
	assert.False(t, report.BudgetsEstablished)
}

func TestEvaluateShadow_HistoricalSingleAggregatedIntervalIsUnresolved(t *testing.T) {
	// Mirrors the current/historical production shape: one aggregated interval with a
	// cumulative count, firstTimestamp in one eligible test and lastTimestamp in another, and
	// no UID/resourceVersion/series. This must be Unresolved: it cannot be attributed and
	// must not create any approval. (Legacy 55/70 behavior is handled elsewhere and untouched.)
	intervals := monitorapi.Intervals{
		e2eInterval(fullT(suffixT1TrustedCA), "Passed", testBase.Add(10*time.Minute), testBase.Add(14*time.Minute)),
		e2eInterval(fullT(suffixT4FullLoginFlow), "Passed", testBase.Add(62*time.Minute), testBase.Add(67*time.Minute)),
		kubeEventInterval(kubeEventOpts{
			namespace: oauthScaleNamespace, deployment: oauthScaleDeployment, reason: oauthScaleReason,
			message: "(combined from similar events): Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3",
			count:   31,
			first:   testBase.Add(11 * time.Minute), // inside T1
			last:    testBase.Add(63 * time.Minute), // inside T4
		}),
	}

	report, junits := EvaluateOAuthBoundedAccountingShadow(intervals, nil)
	assert.Nil(t, junits[0].FailureOutput)
	assert.Equal(t, 1, report.CandidateEvents)
	assert.Equal(t, 0, report.AttributedUnits, "no UID/baseline -> never attributed by lastTimestamp alone")
	assert.Equal(t, 1, report.UnresolvedUnits)
}

// TestEvaluateShadow_HistoricalCountsUnresolved confirms that the historically observed
// cumulative counts (e.g. 56, 64) on a single aggregated interval with no UID/baseline are
// Unresolved and never attributed. We must not claim these counts are now accounted for.
func TestEvaluateShadow_HistoricalCountsUnresolved(t *testing.T) {
	for _, count := range []int{31, 56, 64} {
		t.Run(fmt.Sprintf("count-%d", count), func(t *testing.T) {
			intervals := monitorapi.Intervals{
				e2eInterval(fullT(suffixT1TrustedCA), "Passed", testBase.Add(10*time.Minute), testBase.Add(14*time.Minute)),
				e2eInterval(fullT(suffixT4FullLoginFlow), "Passed", testBase.Add(62*time.Minute), testBase.Add(67*time.Minute)),
				kubeEventInterval(kubeEventOpts{
					namespace: oauthScaleNamespace, deployment: oauthScaleDeployment, reason: oauthScaleReason,
					message: "(combined from similar events): Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3",
					count:   count,
					first:   testBase.Add(11 * time.Minute),
					last:    testBase.Add(63 * time.Minute),
				}),
			}
			report, junits := EvaluateOAuthBoundedAccountingShadow(intervals, nil)
			assert.Nil(t, junits[0].FailureOutput)
			assert.Equal(t, 0, report.AttributedUnits)
			assert.Equal(t, 1, report.UnresolvedUnits)
			assert.Empty(t, report.PerWindowAttributed)
		})
	}
}

// TestEvaluateShadow_BoundedOutput confirms the rendered SystemOut is capped regardless of
// how many candidate events are present (bounded memory/output requirement).
func TestEvaluateShadow_BoundedOutput(t *testing.T) {
	intervals := monitorapi.Intervals{
		e2eInterval(fullT(suffixT4FullLoginFlow), "Passed", testBase.Add(62*time.Minute), testBase.Add(67*time.Minute)),
	}
	// Many distinct no-identity candidates -> many findings, but rendering must stay bounded.
	for i := 0; i < maxRenderedCandidates+25; i++ {
		intervals = append(intervals, kubeEventInterval(kubeEventOpts{
			namespace: oauthScaleNamespace, deployment: oauthScaleDeployment, reason: oauthScaleReason,
			message: "Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3",
			count:   30, last: testBase.Add(63 * time.Minute),
		}))
	}
	_, junits := EvaluateOAuthBoundedAccountingShadow(intervals, nil)
	require.Len(t, junits, 1)
	assert.Nil(t, junits[0].FailureOutput)
	lines := strings.Count(junits[0].SystemOut, "\n")
	assert.LessOrEqual(t, lines, maxRenderedCandidates+20, "rendered output must be bounded")
	assert.Contains(t, junits[0].SystemOut, "suppressed (bounded output)")
}

// TestLegacyAllowancesUnchanged guards that this shadow change does not alter the existing
// AuthenticationComponentProxy allowances (55/70) or their Allows semantics.
func TestLegacyAllowancesUnchanged(t *testing.T) {
	// Build the universal registry the way production does when the component-proxy tests ran.
	intervals := monitorapi.Intervals{
		e2eInterval(fullT(suffixT1TrustedCA), "Passed", testBase.Add(10*time.Minute), testBase.Add(14*time.Minute)),
	}
	registry := NewUniversalPathologicalEventMatchers(nil, intervals)

	scaling, err := registry.GetMatcherByName("AuthenticationComponentProxyScalingReplicaSet")
	require.NoError(t, err)
	deployUpdated, err := registry.GetMatcherByName("AuthenticationComponentProxyDeploymentUpdated")
	require.NoError(t, err)

	sm, ok := scaling.(*SimplePathologicalEventMatcher)
	require.True(t, ok)
	dm, ok := deployUpdated.(*SimplePathologicalEventMatcher)
	require.True(t, ok)

	assert.Equal(t, 55, sm.repeatThresholdOverride, "oauth ScalingReplicaSet allowance must remain 55")
	assert.Equal(t, 70, dm.repeatThresholdOverride, "authentication-operator DeploymentUpdated allowance must remain 70")

	// Allows semantics unchanged: within threshold allowed, over threshold not allowed.
	within := BuildTestDupeKubeEvent(oauthScaleNamespace, "oauth-openshift", oauthScaleReason, "Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3", 50)
	within.Locator.Keys[monitorapi.LocatorDeploymentKey] = "oauth-openshift"
	assert.True(t, sm.Allows(within, ""), "50 <= 55 must still be allowed")

	over := BuildTestDupeKubeEvent(oauthScaleNamespace, "oauth-openshift", oauthScaleReason, "Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3", 60)
	over.Locator.Keys[monitorapi.LocatorDeploymentKey] = "oauth-openshift"
	assert.False(t, sm.Allows(over, ""), "60 > 55 must not be allowed")
}
