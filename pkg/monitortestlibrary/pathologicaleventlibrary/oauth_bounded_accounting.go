package pathologicaleventlibrary

// Bounded OAuth pathological-event accounting (SHADOW).
//
// This file is the OAuth-specific adapter over the domain-agnostic bounded counter accounting
// core in bounded_counter_accounting.go. The core attributes cumulative-counter increments to
// eligible, labelled time windows; this adapter supplies the OAuth specifics: which e2e tests
// are eligible windows, which kube events are candidate oauth rollout-scaling increments, and
// how to render the OAuth shadow report/JUnit. Nothing here changes existing pass/fail
// behavior; it runs in SHADOW and emits an always-passing, informational JUnit plus logs.
//
// Background
//
// The legacy pathological-event monitor flags a kube event as "pathological" when its
// cumulative Count exceeds DuplicateEventThreshold (20), and then forgives it if a registered
// matcher Allows it. For the AuthenticationComponentProxy serial tests the repository forgives
// oauth-openshift ScalingReplicaSet repeats with a flat repeatThresholdOverride (see
// AuthenticationComponentProxyScalingReplicaSet, 55, and AuthenticationComponentProxyDeployment
// Updated, 70, in duplicated_event_patterns.go).
//
// A flat override forgives a single cumulative counter without proving that the counted
// increments actually occurred inside the test windows that are entitled to cause oauth
// rollouts. Production interval data shows why that distinction matters: a single aggregated
// oauth-openshift ScalingReplicaSet interval can carry count=31 with firstTimestamp inside one
// eligible test and lastTimestamp inside a different eligible test, several minutes and other
// tests later. The 31 increments are spread across multiple windows and the gaps between them,
// yet nothing in the interval tells us how.
//
// Honesty constraints (enforced by the generic core)
//
//   - Attribution of an increment requires a reliable baseline (the count immediately before
//     the increment), a stable identity (a kube Event UID), and an increment time span fully
//     contained in a single eligible window. An increment is NEVER attributed from a
//     last-observed time alone.
//   - Historical/current interval data does not carry Event UID, resourceVersion, or
//     per-observation (series) timestamps, and it collapses all repeats of one message into a
//     single aggregated interval, so it classifies as Unresolved. This module does not and
//     cannot reconstruct missing historical UIDs or counter baselines.
//   - Finite budgets are NOT invented. Absent a source-justified budget the shadow result
//     stays Unresolved; it does not fall back to any 55/70/64/10000 number. Activation is a
//     FUTURE, SEPARATE change.
//
// The matching evidence captured for future runs (Event UID, resourceVersion, and series
// count/last-observed time) is recorded conditionally by the kube-event watcher
// (pkg/monitortests/testframework/watchevents/event.go) using the annotation keys below, and
// only when the source event actually carries that data.

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

// Evidence annotation keys. These form the contract between the kube-event watcher (producer)
// and this accounting adapter (consumer). firstTimestamp/lastTimestamp are already written by
// the watcher as raw strings; the remaining keys are new and are only written when the source
// kube Event actually carries the corresponding data.
const (
	AnnotationEventUID            monitorapi.AnnotationKey = "eventUID"
	AnnotationResourceVersion     monitorapi.AnnotationKey = "resourceVersion"
	AnnotationSeriesCount         monitorapi.AnnotationKey = "seriesCount"
	AnnotationSeriesLastObserved  monitorapi.AnnotationKey = "seriesLastObservedTime"
	annotationFirstTimestamp      monitorapi.AnnotationKey = "firstTimestamp"
	annotationLastTimestamp       monitorapi.AnnotationKey = "lastTimestamp"
	oauthShadowAccountingTestName                          = "[sig-auth] oauth-openshift pathological scaling bounded accounting (shadow)"
)

// Candidate kube-event identity. All of these must match exactly for an interval to be a
// candidate oauth rollout scaling event; anything else is ignored (never counted).
const (
	oauthScaleNamespace  = "openshift-authentication"
	oauthScaleDeployment = "oauth-openshift"
	oauthScaleReason     = "ScalingReplicaSet"
)

// maxRenderedCandidates bounds the per-candidate detail lines emitted into the shadow JUnit
// SystemOut so a pathological run cannot produce unbounded output. Totals are always reported;
// only the per-line detail is capped.
const maxRenderedCandidates = 50

// componentProxyTestPrefix is the exact, full locator prefix shared by the
// AuthenticationComponentProxy serial tests.
const componentProxyTestPrefix = `[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial]`

// The exact test-name suffixes (T1..T5). A test is eligible only when its full locator name
// equals componentProxyTestPrefix + " " + one of these suffixes. Note T2 is a strict prefix of
// T1, which is why eligibility is decided by full-name equality, never substring.
const (
	suffixT1TrustedCA     = "operator should validate OIDC IdP through component proxy with trustedCA"
	suffixT2ValidateOIDC  = "operator should validate OIDC IdP through component proxy"
	suffixT3HotReloadCA   = "oauth-server/operator should hot-reload mounted CA file on change when spec.proxy.trustedCA is set"
	suffixT4FullLoginFlow = "oauth-server should perform full OIDC login flow through the proxy when auth proxy config is applied"
	suffixT5FallBack      = "operator should fall back to original configuration on spec.proxy removal"
)

// oauthScaleMessageRegex anchors the oauth-openshift Scaled up/down replica set message and
// parses the integer replica counts. The combined-event prefix is optional and known. The
// replica set name is captured for owner verification (it must be owned by the deployment).
var oauthScaleMessageRegex = regexp.MustCompile(
	`^(?:\(combined from similar events\): )?Scaled (up|down) replica set (oauth-openshift-[a-z0-9]+) from (\d+) to (\d+)$`)

// eligibleTest maps a suffix to its short label and whether a rollout (ScalingReplicaSet) is
// expected during that test. T3 exercises in-place CA hot-reload and must NOT roll the
// deployment, so it maps to a protected (IncrementsExpected=false) window.
type eligibleTest struct {
	label           string
	rolloutExpected bool
}

var componentProxyEligibleTests = map[string]eligibleTest{
	componentProxyTestPrefix + " " + suffixT1TrustedCA:     {label: "T1", rolloutExpected: true},
	componentProxyTestPrefix + " " + suffixT2ValidateOIDC:  {label: "T2", rolloutExpected: true},
	componentProxyTestPrefix + " " + suffixT3HotReloadCA:   {label: "T3", rolloutExpected: false},
	componentProxyTestPrefix + " " + suffixT4FullLoginFlow: {label: "T4", rolloutExpected: true},
	componentProxyTestPrefix + " " + suffixT5FallBack:      {label: "T5", rolloutExpected: true},
}

// OAuthScaleBudgets is the OAuth entry point's budget type. It is an alias of the generic
// CounterBudgets: the algorithm and the window collection are genuinely generic, so there is
// one budget shape shared by every caller. It is nil in production until measured finite
// budgets exist; only MaxIncrementsPerWindow is evaluated today (the generic equivalent of the
// prior per-test runaway safeguard). Activation is a future, separate change.
type OAuthScaleBudgets = CounterBudgets

// OAuthShadowReport is the OAuth adapter's report: the generic AccountingReport plus the
// OAuth-specific candidate/ignored counters.
type OAuthShadowReport struct {
	*AccountingReport
	CandidateEvents int
	IgnoredNonMatch int
}

// oauthEligibleWindows returns the completed, passing AuthenticationComponentProxy test windows
// (T1..T5) as generic EligibleWindows, matched by exact full-name equality. Only "Passed" tests
// are eligible; Flaked/Failed/DidNotFinish/Skipped are not. Eligibility alone is not approval.
func oauthEligibleWindows(intervals monitorapi.Intervals) []EligibleWindow {
	var windows []EligibleWindow
	for _, i := range intervals {
		if i.Source != monitorapi.SourceE2ETest {
			continue
		}
		testName := i.Locator.Keys[monitorapi.LocatorE2ETestKey]
		et, ok := componentProxyEligibleTests[testName]
		if !ok {
			continue
		}
		if i.Message.Annotations[monitorapi.AnnotationStatus] != "Passed" {
			continue
		}
		// A window with a zero or inverted span is not usable.
		if i.To.IsZero() || i.From.IsZero() || !i.To.After(i.From) {
			continue
		}
		windows = append(windows, EligibleWindow{
			Label:              et.label,
			From:               i.From,
			To:                 i.To,
			IncrementsExpected: et.rolloutExpected,
		})
	}
	sort.Slice(windows, func(a, b int) bool { return windows[a].From.Before(windows[b].From) })
	return windows
}

// isCandidateOAuthScaleEvent reports whether an interval is an oauth-openshift rollout scaling
// kube event. Wrong source/resource/reason/namespace/deployment are ignored.
func isCandidateOAuthScaleEvent(i monitorapi.Interval) bool {
	if i.Source != monitorapi.SourceKubeEvent {
		return false
	}
	if string(i.Message.Reason) != oauthScaleReason {
		return false
	}
	if i.Locator.Keys[monitorapi.LocatorNamespaceKey] != oauthScaleNamespace {
		return false
	}
	if i.Locator.Keys[monitorapi.LocatorDeploymentKey] != oauthScaleDeployment {
		return false
	}
	return true
}

// oauthScaleMessageParse holds the parsed oauth scaling message fields.
type oauthScaleMessageParse struct {
	direction     string
	replicaSet    string
	fromReplicas  int
	toReplicas    int
	ownerVerified bool
	ok            bool
}

// parseOAuthScaleMessage parses and owner-verifies the anchored oauth scaling human message.
// ok is false when the message does not match the anchored pattern; ownerVerified confirms the
// replica set is owned by the deployment ("<deployment>-<hash>").
func parseOAuthScaleMessage(human string) oauthScaleMessageParse {
	m := oauthScaleMessageRegex.FindStringSubmatch(human)
	if m == nil {
		return oauthScaleMessageParse{}
	}
	from, errFrom := strconv.Atoi(m[3])
	to, errTo := strconv.Atoi(m[4])
	if errFrom != nil || errTo != nil {
		return oauthScaleMessageParse{}
	}
	return oauthScaleMessageParse{
		direction:     m[1],
		replicaSet:    m[2],
		fromReplicas:  from,
		toReplicas:    to,
		ownerVerified: strings.HasPrefix(m[2], oauthScaleDeployment+"-"),
		ok:            true,
	}
}

// oauthScaleObservations converts candidate oauth scaling intervals into generic
// CounterObservations. candidates counts owner-verified, parseable candidates; ignored counts
// kube events that carry the scaling reason but fail the exact candidate/owner/message checks.
func oauthScaleObservations(intervals monitorapi.Intervals) (obs []CounterObservation, candidates, ignored int) {
	for _, i := range intervals {
		if !isCandidateOAuthScaleEvent(i) {
			if i.Source == monitorapi.SourceKubeEvent && string(i.Message.Reason) == oauthScaleReason {
				ignored++
			}
			continue
		}
		p := parseOAuthScaleMessage(i.Message.HumanMessage)
		if !p.ok || !p.ownerVerified {
			// Matched the identity but not the anchored, owner-verified message: do not count it
			// as a rollout increment; record it as ignored non-matching.
			ignored++
			continue
		}
		candidates++

		ann := i.Message.Annotations
		o := CounterObservation{
			Identity: ann[AnnotationEventUID],
			Count:    GetTimesAnEventHappened(i.Message),
			Note: fmt.Sprintf("rs=%s scaled %s %d->%d replicas",
				p.replicaSet, p.direction, p.fromReplicas, p.toReplicas),
		}
		if rvStr := ann[AnnotationResourceVersion]; rvStr != "" {
			if rv, err := strconv.ParseInt(rvStr, 10, 64); err == nil {
				o.Sequence = rv
				o.HasSequence = true
			}
		}
		// Attribution requires a reliable sub-second observation time. Only the RFC3339Nano
		// seriesLastObservedTime qualifies; we deliberately do NOT fall back to the second-
		// precision lastTimestamp annotation, because an event observed at window-end+500ms would
		// serialize to the end second and be wrongly treated as contained. When no valid series
		// time is present, Observed stays zero and the core classifies the step Unresolved
		// (fail-closed). The first/last timestamp annotations remain untouched for charting/legacy.
		o.Observed = parseAnnotatedTime(ann[AnnotationSeriesLastObserved])
		obs = append(obs, o)
	}
	return obs, candidates, ignored
}

func parseAnnotatedTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

// EvaluateOAuthBoundedAccountingShadow runs the shadow accounting over the final intervals and
// returns a bounded report plus an always-passing informational JUnit. It NEVER fails a run and
// NEVER alters any existing verdict or allowance. budgets is nil in production.
func EvaluateOAuthBoundedAccountingShadow(intervals monitorapi.Intervals, budgets *OAuthScaleBudgets) (*OAuthShadowReport, []*junitapi.JUnitTestCase) {
	windows := oauthEligibleWindows(intervals)
	obs, candidates, ignored := oauthScaleObservations(intervals)

	rep := AccountByIdentity(obs, windows, budgets)

	out := &OAuthShadowReport{
		AccountingReport: rep,
		CandidateEvents:  candidates,
		IgnoredNonMatch:  ignored,
	}
	return out, []*junitapi.JUnitTestCase{out.toJUnit(budgets)}
}

// toJUnit renders the shadow report as a single, always-passing JUnit test case with a bounded
// SystemOut. It is informational only.
func (r *OAuthShadowReport) toJUnit(budgets *OAuthScaleBudgets) *junitapi.JUnitTestCase {
	var b strings.Builder
	fmt.Fprintf(&b, "SHADOW oauth-openshift ScalingReplicaSet bounded accounting (informational; does not affect pass/fail)\n")
	fmt.Fprintf(&b, "eligible passing windows (T1-T5): %d\n", r.EligibleWindows)
	fmt.Fprintf(&b, "candidate oauth scaling events: %d (ignored non-matching: %d)\n", r.CandidateEvents, r.IgnoredNonMatch)
	fmt.Fprintf(&b, "attributed increments: %d; unresolved units: %d; protected (no-rollout) units: %d\n",
		r.AttributedUnits, r.UnresolvedUnits, r.ProtectedUnits)
	if budgets == nil {
		fmt.Fprintf(&b, "finite budgets: NONE established -> no shadow decision (attribution reported only; activation is a future, separate change)\n")
	} else {
		fmt.Fprintf(&b, "finite budgets: established (per-window=%d total=%d)\n", budgets.MaxIncrementsPerWindow, budgets.MaxIncrementsTotal)
	}
	if len(r.PerWindowAttributed) > 0 {
		labels := make([]string, 0, len(r.PerWindowAttributed))
		for l := range r.PerWindowAttributed {
			labels = append(labels, l)
		}
		sort.Strings(labels)
		for _, l := range labels {
			fmt.Fprintf(&b, "  attributed[%s]=%d\n", l, r.PerWindowAttributed[l])
		}
	}
	rendered := 0
	for _, f := range r.Findings {
		if rendered >= maxRenderedCandidates {
			fmt.Fprintf(&b, "  ... %d more findings suppressed (bounded output)\n", len(r.Findings)-rendered)
			break
		}
		fmt.Fprintf(&b, "  [%s] id=%q inc=%d window=%s %s\n", f.Class, f.Identity, f.Increment, f.Window, f.Detail)
		rendered++
	}

	logrus.WithFields(logrus.Fields{
		"eligibleWindows": r.EligibleWindows,
		"candidates":      r.CandidateEvents,
		"attributed":      r.AttributedUnits,
		"unresolved":      r.UnresolvedUnits,
		"protected":       r.ProtectedUnits,
		"budgets":         r.BudgetsEstablished,
	}).Info("oauth bounded accounting shadow result")

	// No FailureOutput: this JUnit always passes. Shadow decisions never alter verdicts.
	return &junitapi.JUnitTestCase{
		Name:      oauthShadowAccountingTestName,
		SystemOut: b.String(),
	}
}
