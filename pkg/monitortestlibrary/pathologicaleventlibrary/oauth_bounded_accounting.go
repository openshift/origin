package pathologicaleventlibrary

// Bounded OAuth pathological-event accounting (SHADOW).
//
// Background
//
// The legacy pathological-event monitor flags a kube event as "pathological" when its
// cumulative Count exceeds DuplicateEventThreshold (20), and then forgives it if a
// registered matcher Allows it. For the AuthenticationComponentProxy serial tests the
// repository currently forgives oauth-openshift ScalingReplicaSet repeats with a flat
// repeatThresholdOverride (see AuthenticationComponentProxyScalingReplicaSet, 55, and
// AuthenticationComponentProxyDeploymentUpdated, 70, in duplicated_event_patterns.go).
//
// A flat override forgives a single cumulative counter without proving that the counted
// increments actually occurred inside the test windows that are entitled to cause oauth
// rollouts. Production interval data shows why that distinction matters: a single
// aggregated oauth-openshift ScalingReplicaSet interval can carry count=31 with
// firstTimestamp inside one eligible test and lastTimestamp inside a different eligible
// test, several minutes and other tests later. The 31 increments are spread across
// multiple windows and the gaps between them, yet nothing in the interval tells us how.
//
// What this module does (and does not do)
//
// This module implements the accounting and evidence model needed to attribute oauth
// scaling increments to the specific eligible test windows that are entitled to them,
// WITHOUT changing any existing pass/fail behavior. It runs in SHADOW: it never calls
// into the allowance registry, never forgives anything, and its result is emitted only as
// an always-passing, informational JUnit plus logs. The existing 55/70 allowances and the
// "[sig-arch] events should not repeat pathologically" verdict are untouched.
//
// Honesty constraints (deliberate)
//
//   - Attribution of an increment requires three things together: a reliable baseline
//     (the count immediately before the increment), a stable identity (a kube Event UID),
//     and an increment time span fully contained in a single eligible window. An increment
//     is NEVER attributed from a lastTimestamp alone.
//   - Historical/current interval data does not carry Event UID, resourceVersion, or
//     per-observation (series) timestamps, and it collapses all repeats of one message into
//     a single aggregated interval. Such data yields no reliable baseline and no stable
//     per-observation identity, so it classifies as Unresolved. This module does not and
//     cannot reconstruct missing historical UIDs or counter baselines.
//   - Finite per-mutation / per-test / per-suite budgets are NOT invented here. Absent a
//     source-justified budget configuration the shadow result stays Unresolved; it does not
//     fall back to any 55/70/64/10000 number. Establishing measured finite budgets and then
//     activating enforcement is a FUTURE, SEPARATE change.
//
// The matching evidence captured for future runs (Event UID, resourceVersion, and series
// count/last-observed time) is recorded conditionally by the kube-event watcher
// (pkg/monitortests/testframework/watchevents/event.go) using the annotation keys defined
// below, and only when the source event actually carries that data.

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

// Evidence annotation keys. These form the contract between the kube-event watcher
// (producer) and this accounting module (consumer). firstTimestamp/lastTimestamp are
// already written by the watcher as raw strings; the remaining keys are new and are only
// written when the source kube Event actually carries the corresponding data.
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

// boundaryToleranceSeconds is the slack applied when testing whether an increment span is
// contained in a window. It is intentionally 0 (strict, zero post-test grace). Interval
// timestamps are RFC3339 (second precision); loosening this has not been shown necessary
// and would weaken containment, so it stays strict and is covered by boundary tests.
const boundaryToleranceSeconds = 0

// maxRenderedCandidates bounds the per-candidate detail lines emitted into the shadow
// JUnit SystemOut so a pathological run cannot produce unbounded output. Totals are always
// reported; only the per-line detail is capped.
const maxRenderedCandidates = 50

// componentProxyTestPrefix is the exact, full locator prefix shared by the
// AuthenticationComponentProxy serial tests.
const componentProxyTestPrefix = `[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial]`

// The exact test-name suffixes (T1..T5). A test is eligible only when its full locator
// name equals componentProxyTestPrefix + " " + one of these suffixes. Note T2 is a strict
// prefix of T1, which is why eligibility is decided by full-name equality, never substring.
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

// eligibleTest maps a suffix to its short label and whether a rollout (ScalingReplicaSet)
// is expected during that test. T3 exercises in-place CA hot-reload and must NOT roll the
// deployment, so it is a protected no-rollout window: increments contained in T3 are never
// attributed/forgiven, they are surfaced as a protective Unresolved classification.
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

// OAuthScaleBudgets models finite, source-justified budgets and rate/duration safeguards.
// It is nil in production until measured finite budgets exist; while nil, attributed
// increments are reported but produce no shadow decision (they stay Unresolved at the
// budget layer). This type exists so the future activation change is a configuration
// change, not a redesign. No default numbers are supplied on purpose.
//
// Shadow wiring note: only MaxIncrementsPerTest is evaluated today (it demonstrates the
// per-test runaway safeguard). MaxIncrementsPerSuite, MaxIncrementsPerMinute and
// MaxWindowDuration are the reserved contract for the future activation change and are not
// yet evaluated; they are documented here so the shape is stable.
type OAuthScaleBudgets struct {
	// MaxIncrementsPerTest bounds attributed increments within a single eligible test.
	// Evaluated in the current shadow.
	MaxIncrementsPerTest int
	// MaxIncrementsPerSuite bounds attributed increments across all eligible tests. Reserved.
	MaxIncrementsPerSuite int
	// MaxIncrementsPerMinute is a rate safeguard over an attributed increment span. Reserved.
	MaxIncrementsPerMinute float64
	// MaxWindowDuration is a duration safeguard; spans longer than this are suspect. Reserved.
	MaxWindowDuration time.Duration
}

// oauthEligibleWindow is one eligible, completed, passing test window.
type oauthEligibleWindow struct {
	testName        string
	label           string
	rolloutExpected bool
	from            time.Time
	to              time.Time
}

// oauthScaleObservation is one observation of a candidate oauth scaling kube event. It holds
// only what the accounting classifier consumes; additional captured evidence (e.g. series
// count, firstTimestamp) lives on the interval annotations for artifacts and future use.
type oauthScaleObservation struct {
	uid             string
	resourceVersion int64
	hasRV           bool
	count           int
	last            time.Time
	observed        time.Time // series last-observed time when present, else last
	direction       string
	replicaSet      string
	fromReplicas    int
	toReplicas      int
	parseOK         bool
	ownerVerified   bool
}

// oauthScaleClass is the classification of an accounting unit (a per-identity increment
// step, or a lone observation that cannot form a step).
type oauthScaleClass string

const (
	classAttributed               oauthScaleClass = "Attributed"
	classUnresolvedNoWindow       oauthScaleClass = "UnresolvedNoEligibleWindow"
	classUnresolvedSpanCrosses    oauthScaleClass = "UnresolvedSpanCrossesWindowOrGap"
	classUnresolvedNoBaseline     oauthScaleClass = "UnresolvedNoReliableBaseline"
	classUnresolvedNoIdentity     oauthScaleClass = "UnresolvedNoStableIdentity"
	classUnresolvedMalformed      oauthScaleClass = "UnresolvedMalformedOrOutOfOrder"
	classProtectedNoRolloutPhase  oauthScaleClass = "UnresolvedProtectedNoRolloutPhase"
	classUnresolvedBudgetExceeded oauthScaleClass = "UnresolvedBudgetExceeded"
)

// oauthScaleFinding is one classified accounting unit for diagnostics.
type oauthScaleFinding struct {
	class     oauthScaleClass
	uid       string
	increment int
	spanFrom  time.Time
	spanTo    time.Time
	window    string // label of the window it was attributed/compared to, when any
	detail    string
}

// OAuthShadowReport is the bounded, serialization-friendly summary of a shadow run.
type OAuthShadowReport struct {
	EligibleWindows     int
	CandidateEvents     int
	IgnoredNonMatch     int
	AttributedUnits     int
	UnresolvedUnits     int
	ProtectedT3Units    int
	BudgetsEstablished  bool
	PerWindowAttributed map[string]int
	Findings            []oauthScaleFinding
}

// collectEligibleWindows returns the completed, passing AuthenticationComponentProxy test
// windows (T1..T5), matched by exact full-name equality. Only "Passed" tests are eligible;
// Flaked/Failed/DidNotFinish/Skipped are not. Eligibility alone is not approval.
func collectEligibleWindows(intervals monitorapi.Intervals) []oauthEligibleWindow {
	var windows []oauthEligibleWindow
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
		windows = append(windows, oauthEligibleWindow{
			testName:        testName,
			label:           et.label,
			rolloutExpected: et.rolloutExpected,
			from:            i.From,
			to:              i.To,
		})
	}
	sort.Slice(windows, func(a, b int) bool { return windows[a].from.Before(windows[b].from) })
	return windows
}

// isCandidateOAuthScaleEvent reports whether an interval is an oauth-openshift rollout
// scaling kube event. Wrong source/resource/reason/namespace/deployment are ignored.
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

// parseObservation turns a candidate interval into an observation, parsing the message and
// evidence annotations. parseOK is false when the message does not match the anchored
// pattern. Missing evidence (UID/resourceVersion/series) is preserved as absent, not
// invented.
func parseObservation(i monitorapi.Interval) oauthScaleObservation {
	ann := i.Message.Annotations
	obs := oauthScaleObservation{
		uid:   ann[AnnotationEventUID],
		count: GetTimesAnEventHappened(i.Message),
	}
	if rvStr := ann[AnnotationResourceVersion]; rvStr != "" {
		if rv, err := strconv.ParseInt(rvStr, 10, 64); err == nil {
			obs.resourceVersion = rv
			obs.hasRV = true
		}
	}
	obs.last = parseAnnotatedTime(ann[annotationLastTimestamp])
	obs.observed = parseAnnotatedTime(ann[AnnotationSeriesLastObserved])
	if obs.observed.IsZero() {
		obs.observed = obs.last
	}

	m := oauthScaleMessageRegex.FindStringSubmatch(i.Message.HumanMessage)
	if m == nil {
		return obs
	}
	from, errFrom := strconv.Atoi(m[3])
	to, errTo := strconv.Atoi(m[4])
	if errFrom != nil || errTo != nil {
		return obs
	}
	obs.direction = m[1]
	obs.replicaSet = m[2]
	obs.fromReplicas = from
	obs.toReplicas = to
	obs.parseOK = true
	// Owner verification where possible: the replica set must be owned by the deployment,
	// i.e. named "<deployment>-<hash>".
	obs.ownerVerified = strings.HasPrefix(obs.replicaSet, oauthScaleDeployment+"-")
	return obs
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

// windowContaining returns the single eligible window that fully contains [spanFrom,spanTo]
// (with strict boundaryToleranceSeconds slack). containedBy is set only when exactly one
// window fully contains the span. overlapsAny reports whether the span intersects (even by
// touching a boundary) any eligible window, which lets callers tell a boundary-crossing span
// apart from a span that falls entirely in a gap with no eligible window at all.
func windowContaining(windows []oauthEligibleWindow, spanFrom, spanTo time.Time) (containing oauthEligibleWindow, containedBy int, overlapsAny bool) {
	tol := time.Duration(boundaryToleranceSeconds) * time.Second
	for _, w := range windows {
		fullyContains := !spanFrom.Before(w.from.Add(-tol)) && !spanTo.After(w.to.Add(tol))
		if fullyContains {
			containing = w
			containedBy++
		}
		// Intersection (inclusive of touching boundaries).
		if !spanFrom.After(w.to) && !spanTo.Before(w.from) {
			overlapsAny = true
		}
	}
	return containing, containedBy, overlapsAny
}

// accountIdentityGroup performs bounded accounting over the ordered observations of a
// single kube Event identity (UID). Adjacent observations form increment steps; each step
// has a real baseline (the previous count) and a real time span (previous observed ->
// current observed). Dedup, reset, collision, out-of-order and malformed cases are handled
// explicitly. Each step is classified against the eligible windows and (optionally) the
// budgets. budgets may be nil (no established budget -> no decision).
func accountIdentityGroup(uid string, obs []oauthScaleObservation, windows []oauthEligibleWindow, budgets *OAuthScaleBudgets) []oauthScaleFinding {
	var findings []oauthScaleFinding

	// Order by resourceVersion when present, else by observed time. Keep a stable order.
	sort.SliceStable(obs, func(a, b int) bool {
		if obs[a].hasRV && obs[b].hasRV && obs[a].resourceVersion != obs[b].resourceVersion {
			return obs[a].resourceVersion < obs[b].resourceVersion
		}
		return obs[a].observed.Before(obs[b].observed)
	})

	// Dedup identical (uid,resourceVersion) observations; detect collisions (same rv,
	// different count).
	deduped := make([]oauthScaleObservation, 0, len(obs))
	for _, o := range obs {
		if len(deduped) > 0 {
			prev := deduped[len(deduped)-1]
			if o.hasRV && prev.hasRV && o.resourceVersion == prev.resourceVersion {
				if o.count != prev.count {
					findings = append(findings, oauthScaleFinding{
						class: classUnresolvedMalformed, uid: uid, increment: 0,
						detail: fmt.Sprintf("resourceVersion %d observed with conflicting counts %d and %d", o.resourceVersion, prev.count, o.count),
					})
				}
				continue // duplicate resourceVersion: skip
			}
		}
		deduped = append(deduped, o)
	}

	if len(deduped) < 2 {
		// A single observation cannot establish a baseline; attributing it would rely on a
		// cumulative counter and a lastTimestamp alone, which is exactly what we refuse.
		class := classUnresolvedNoBaseline
		if uid == "" {
			class = classUnresolvedNoIdentity
		}
		inc := 0
		if len(deduped) == 1 {
			inc = deduped[0].count
		}
		findings = append(findings, oauthScaleFinding{
			class: class, uid: uid, increment: inc,
			detail: "only one observation for this identity; no reliable baseline (never attribute by lastTimestamp alone)",
		})
		return findings
	}

	perTest := map[string]int{}
	for idx := 1; idx < len(deduped); idx++ {
		prev := deduped[idx-1]
		cur := deduped[idx]

		// Reset: the cumulative counter dropped. Start a fresh baseline from this point; the
		// step itself is unresolved (we cannot know the real increment across a reset).
		if cur.count < prev.count {
			findings = append(findings, oauthScaleFinding{
				class: classUnresolvedMalformed, uid: uid, increment: 0,
				detail: fmt.Sprintf("counter reset detected (%d -> %d); baseline restarted", prev.count, cur.count),
			})
			continue
		}

		// Out-of-order: observed time goes backward across a forward resourceVersion step.
		if cur.observed.Before(prev.observed) {
			findings = append(findings, oauthScaleFinding{
				class: classUnresolvedMalformed, uid: uid, increment: cur.count - prev.count,
				detail: "observation time decreased across a forward step (out-of-order/late observation)",
			})
			continue
		}

		increment := cur.count - prev.count
		if increment == 0 {
			continue // no new occurrences between observations
		}

		// The increment occurred somewhere in (prev.observed, cur.observed]. We require the
		// whole span to be contained in a single eligible window; baseline is prev.count.
		spanFrom := prev.observed
		spanTo := cur.observed
		if spanFrom.IsZero() || spanTo.IsZero() {
			findings = append(findings, oauthScaleFinding{
				class: classUnresolvedNoBaseline, uid: uid, increment: increment,
				detail: "missing observation timestamps; cannot bound the increment span",
			})
			continue
		}

		w, containedBy, overlapsAny := windowContaining(windows, spanFrom, spanTo)
		if containedBy != 1 {
			class := classUnresolvedSpanCrosses
			detail := "increment span is not fully contained in exactly one eligible window"
			if containedBy == 0 && !overlapsAny {
				class = classUnresolvedNoWindow
				detail = "increment span falls outside every eligible window"
			}
			findings = append(findings, oauthScaleFinding{
				class: class, uid: uid, increment: increment,
				spanFrom: spanFrom, spanTo: spanTo,
				detail: detail,
			})
			continue
		}
		if !w.rolloutExpected {
			// Protected no-rollout phase (T3): scaling here is never forgiven.
			findings = append(findings, oauthScaleFinding{
				class: classProtectedNoRolloutPhase, uid: uid, increment: increment,
				spanFrom: spanFrom, spanTo: spanTo, window: w.label,
				detail: "increment contained in a no-rollout test window; not attributable/forgivable",
			})
			continue
		}

		// Reliable baseline + stable identity + fully-contained span in a rollout-expected
		// window: this increment is attributable for accounting purposes.
		perTest[w.label] += increment
		findings = append(findings, oauthScaleFinding{
			class: classAttributed, uid: uid, increment: increment,
			spanFrom: spanFrom, spanTo: spanTo, window: w.label,
			detail: fmt.Sprintf("baseline=%d step=%d->%d rs=%s scaled %s %d->%d replicas",
				prev.count, prev.count, cur.count, cur.replicaSet, cur.direction, cur.fromReplicas, cur.toReplicas),
		})
	}

	// Budget layer. Attribution does not imply approval. Without an established budget there
	// is no shadow decision; the attributed increments remain Unresolved at the budget layer.
	if budgets == nil {
		return findings
	}
	for label, total := range perTest {
		if budgets.MaxIncrementsPerTest > 0 && total > budgets.MaxIncrementsPerTest {
			findings = append(findings, oauthScaleFinding{
				class: classUnresolvedBudgetExceeded, uid: uid, increment: total, window: label,
				detail: fmt.Sprintf("attributed %d increments exceed per-test budget %d", total, budgets.MaxIncrementsPerTest),
			})
		}
	}
	return findings
}

// EvaluateOAuthBoundedAccountingShadow runs the shadow accounting over the final intervals
// and returns a bounded report plus an always-passing informational JUnit. It NEVER fails a
// run and NEVER alters any existing verdict or allowance. budgets is nil in production.
func EvaluateOAuthBoundedAccountingShadow(intervals monitorapi.Intervals, budgets *OAuthScaleBudgets) (*OAuthShadowReport, []*junitapi.JUnitTestCase) {
	report := &OAuthShadowReport{
		PerWindowAttributed: map[string]int{},
		BudgetsEstablished:  budgets != nil,
	}

	windows := collectEligibleWindows(intervals)
	report.EligibleWindows = len(windows)

	// Group candidate observations by stable identity (UID). Observations without a UID
	// cannot be grouped reliably and are accounted individually as no-identity.
	byUID := map[string][]oauthScaleObservation{}
	var noIdentity []oauthScaleObservation
	for _, i := range intervals {
		if !isCandidateOAuthScaleEvent(i) {
			if i.Source == monitorapi.SourceKubeEvent && string(i.Message.Reason) == oauthScaleReason {
				report.IgnoredNonMatch++
			}
			continue
		}
		obs := parseObservation(i)
		if !obs.parseOK || !obs.ownerVerified {
			// Matched the identity but not the anchored, owner-verified message: do not count
			// it as a rollout increment; record it as ignored non-matching.
			report.IgnoredNonMatch++
			continue
		}
		report.CandidateEvents++
		if obs.uid == "" {
			noIdentity = append(noIdentity, obs)
			continue
		}
		byUID[obs.uid] = append(byUID[obs.uid], obs)
	}

	// Iterate identities in a stable order so the bounded, capped output is reproducible
	// across runs (map iteration order is otherwise random).
	uids := make([]string, 0, len(byUID))
	for uid := range byUID {
		uids = append(uids, uid)
	}
	sort.Strings(uids)

	var allFindings []oauthScaleFinding
	for _, uid := range uids {
		allFindings = append(allFindings, accountIdentityGroup(uid, byUID[uid], windows, budgets)...)
	}
	for _, o := range noIdentity {
		allFindings = append(allFindings, accountIdentityGroup("", []oauthScaleObservation{o}, windows, budgets)...)
	}
	sort.SliceStable(allFindings, func(a, b int) bool {
		if allFindings[a].window != allFindings[b].window {
			return allFindings[a].window < allFindings[b].window
		}
		if allFindings[a].class != allFindings[b].class {
			return allFindings[a].class < allFindings[b].class
		}
		if allFindings[a].uid != allFindings[b].uid {
			return allFindings[a].uid < allFindings[b].uid
		}
		return allFindings[a].spanFrom.Before(allFindings[b].spanFrom)
	})

	for _, f := range allFindings {
		switch f.class {
		case classAttributed:
			report.AttributedUnits++
			if f.window != "" {
				report.PerWindowAttributed[f.window] += f.increment
			}
		case classProtectedNoRolloutPhase:
			report.ProtectedT3Units++
		default:
			report.UnresolvedUnits++
		}
	}
	report.Findings = allFindings

	return report, []*junitapi.JUnitTestCase{report.toJUnit(budgets)}
}

// toJUnit renders the shadow report as a single, always-passing JUnit test case with a
// bounded SystemOut. It is informational only.
func (r *OAuthShadowReport) toJUnit(budgets *OAuthScaleBudgets) *junitapi.JUnitTestCase {
	var b strings.Builder
	fmt.Fprintf(&b, "SHADOW oauth-openshift ScalingReplicaSet bounded accounting (informational; does not affect pass/fail)\n")
	fmt.Fprintf(&b, "eligible passing windows (T1-T5): %d\n", r.EligibleWindows)
	fmt.Fprintf(&b, "candidate oauth scaling events: %d (ignored non-matching: %d)\n", r.CandidateEvents, r.IgnoredNonMatch)
	fmt.Fprintf(&b, "attributed increments: %d; unresolved units: %d; protected T3 (no-rollout) units: %d\n",
		r.AttributedUnits, r.UnresolvedUnits, r.ProtectedT3Units)
	if budgets == nil {
		fmt.Fprintf(&b, "finite budgets: NONE established -> no shadow decision (attribution reported only; activation is a future, separate change)\n")
	} else {
		fmt.Fprintf(&b, "finite budgets: established (per-test=%d per-suite=%d)\n", budgets.MaxIncrementsPerTest, budgets.MaxIncrementsPerSuite)
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
		fmt.Fprintf(&b, "  [%s] uid=%q inc=%d window=%s %s\n", f.class, f.uid, f.increment, f.window, f.detail)
		rendered++
	}

	logrus.WithFields(logrus.Fields{
		"eligibleWindows": r.EligibleWindows,
		"candidates":      r.CandidateEvents,
		"attributed":      r.AttributedUnits,
		"unresolved":      r.UnresolvedUnits,
		"protectedT3":     r.ProtectedT3Units,
		"budgets":         r.BudgetsEstablished,
	}).Info("oauth bounded accounting shadow result")

	// No FailureOutput: this JUnit always passes. Shadow decisions never alter verdicts.
	return &junitapi.JUnitTestCase{
		Name:      oauthShadowAccountingTestName,
		SystemOut: b.String(),
	}
}
