package pathologicaleventlibrary

// Bounded counter accounting (domain-agnostic core).
//
// This is the reusable algorithm behind the oauth shadow accounting (see
// oauth_bounded_accounting.go). It answers one question for any domain: given a cumulative
// counter observed over time under a stable identity, how many increments can be reliably
// attributed to a set of eligible, labelled time windows?
//
// It is intentionally decoupled from any specific resource, test suite, or kube event. A
// caller adapts its domain by producing:
//   - []EligibleWindow  — the labelled windows during which increments are expected, plus
//     "protected" windows where increments are explicitly NOT expected.
//   - []CounterObservation — the observed counter samples (identity, ordering, count, time).
// and then calls AccountByIdentity. The oauth adapter is one such caller; a second,
// non-oauth caller is exercised in the tests to keep this core honestly generic.
//
// Attribution rules (deliberately strict; shared by every caller):
//   - An increment is the delta between two adjacent observations of the SAME identity.
//   - It is attributed to a window only when a reliable baseline exists (the previous
//     observation's count), the identity is stable, and the whole [prev.Observed,
//     cur.Observed] span is fully contained in exactly one increments-expected window.
//   - Resets, out-of-order/late arrivals, collisions, missing timestamps, boundary-crossing
//     spans, and protected (no-increment) windows are never attributed.
//   - Budgets are optional. Absent budgets there is no decision; attribution is reported but
//     nothing is forgiven.

import (
	"fmt"
	"sort"
	"time"
)

// boundaryToleranceSeconds is the slack applied when testing whether an increment span is
// contained in a window. It is intentionally 0 (strict). Observation times may be truncated
// by their serialization, so loosening this would weaken containment; it stays strict and is
// covered by boundary tests.
const boundaryToleranceSeconds = 0

// EligibleWindow is one labelled time window. IncrementsExpected distinguishes a window where
// increments are legitimate (e.g. a test that intentionally triggers rollouts) from a
// protected window where increments must never be attributed/forgiven (e.g. a test that must
// NOT cause a rollout).
type EligibleWindow struct {
	Label              string
	From               time.Time
	To                 time.Time
	IncrementsExpected bool
}

// CounterObservation is one observation of a cumulative counter.
type CounterObservation struct {
	// Identity groups observations that belong to the same underlying counter (e.g. a kube
	// Event UID). An empty Identity means "no stable identity": such observations are never
	// paired into increment steps.
	Identity string
	// Sequence is a monotonic ordering key when present (e.g. resourceVersion). When absent,
	// observations are ordered by Observed.
	Sequence    int64
	HasSequence bool
	// Count is the cumulative counter value at this observation.
	Count int
	// Observed is the time this observation was recorded.
	Observed time.Time
	// Note is opaque domain context surfaced in attributed diagnostics (never interpreted).
	Note string
}

// CounterBudgets models finite, source-justified budgets and rate/duration safeguards. It is
// nil until measured finite budgets exist; while nil, attributed increments are reported but
// produce no decision. No default numbers are supplied on purpose.
//
// Only MaxIncrementsPerWindow is evaluated today (the per-window runaway safeguard).
// MaxIncrementsTotal, MaxIncrementsPerMinute and MaxWindowDuration are the reserved contract
// for a future activation change and are not yet evaluated.
type CounterBudgets struct {
	// MaxIncrementsPerWindow bounds attributed increments within a single window. Evaluated.
	MaxIncrementsPerWindow int
	// MaxIncrementsTotal bounds attributed increments across all windows. Reserved.
	MaxIncrementsTotal int
	// MaxIncrementsPerMinute is a rate safeguard over an attributed increment span. Reserved.
	MaxIncrementsPerMinute float64
	// MaxWindowDuration is a duration safeguard; spans longer than this are suspect. Reserved.
	MaxWindowDuration time.Duration
}

// AccountingClass is the classification of an accounting unit (a per-identity increment step,
// or a lone observation that cannot form a step).
type AccountingClass string

const (
	ClassAttributed               AccountingClass = "Attributed"
	ClassUnresolvedNoWindow       AccountingClass = "UnresolvedNoEligibleWindow"
	ClassUnresolvedSpanCrosses    AccountingClass = "UnresolvedSpanCrossesWindowOrGap"
	ClassUnresolvedNoBaseline     AccountingClass = "UnresolvedNoReliableBaseline"
	ClassUnresolvedNoIdentity     AccountingClass = "UnresolvedNoStableIdentity"
	ClassUnresolvedMalformed      AccountingClass = "UnresolvedMalformedOrOutOfOrder"
	ClassProtectedWindow          AccountingClass = "UnresolvedProtectedWindow"
	ClassUnresolvedBudgetExceeded AccountingClass = "UnresolvedBudgetExceeded"
)

// AccountingFinding is one classified accounting unit for diagnostics.
type AccountingFinding struct {
	Class     AccountingClass
	Identity  string
	Increment int
	SpanFrom  time.Time
	SpanTo    time.Time
	Window    string // label of the window it was attributed/compared to, when any
	Detail    string
}

// AccountingReport is the bounded, serialization-friendly summary of a run.
type AccountingReport struct {
	EligibleWindows     int
	AttributedUnits     int
	UnresolvedUnits     int
	ProtectedUnits      int
	BudgetsEstablished  bool
	PerWindowAttributed map[string]int
	Findings            []AccountingFinding
}

// windowContaining returns the single window that fully contains [spanFrom,spanTo] (with
// strict boundaryToleranceSeconds slack). containedBy is set only when exactly one window
// fully contains the span. overlapsAny reports whether the span intersects (even by touching
// a boundary) any window, which lets callers tell a boundary-crossing span apart from a span
// that falls entirely in a gap with no eligible window at all.
func windowContaining(windows []EligibleWindow, spanFrom, spanTo time.Time) (containing EligibleWindow, containedBy int, overlapsAny bool) {
	tol := time.Duration(boundaryToleranceSeconds) * time.Second
	for _, w := range windows {
		fullyContains := !spanFrom.Before(w.From.Add(-tol)) && !spanTo.After(w.To.Add(tol))
		if fullyContains {
			containing = w
			containedBy++
		}
		// Intersection (inclusive of touching boundaries).
		if !spanFrom.After(w.To) && !spanTo.Before(w.From) {
			overlapsAny = true
		}
	}
	return containing, containedBy, overlapsAny
}

// accountIdentityGroup performs bounded accounting over the observations of a single identity.
// Adjacent observations form increment steps; each step has a real baseline (the previous
// count) and a real time span (previous observed -> current observed). Dedup, reset,
// collision, out-of-order and malformed cases are handled explicitly. budgets may be nil.
func accountIdentityGroup(identity string, obs []CounterObservation, windows []EligibleWindow, budgets *CounterBudgets) []AccountingFinding {
	var findings []AccountingFinding

	// Order by sequence when present, else by observed time. Keep a stable order.
	sort.SliceStable(obs, func(a, b int) bool {
		if obs[a].HasSequence && obs[b].HasSequence && obs[a].Sequence != obs[b].Sequence {
			return obs[a].Sequence < obs[b].Sequence
		}
		return obs[a].Observed.Before(obs[b].Observed)
	})

	// Dedup identical (identity,sequence) observations; detect collisions (same sequence,
	// different count).
	deduped := make([]CounterObservation, 0, len(obs))
	for _, o := range obs {
		if len(deduped) > 0 {
			prev := deduped[len(deduped)-1]
			if o.HasSequence && prev.HasSequence && o.Sequence == prev.Sequence {
				if o.Count != prev.Count {
					findings = append(findings, AccountingFinding{
						Class: ClassUnresolvedMalformed, Identity: identity, Increment: 0,
						Detail: fmt.Sprintf("sequence %d observed with conflicting counts %d and %d", o.Sequence, prev.Count, o.Count),
					})
				}
				continue // duplicate sequence: skip
			}
		}
		deduped = append(deduped, o)
	}

	if len(deduped) < 2 {
		// A single observation cannot establish a baseline; attributing it would rely on a
		// cumulative counter and a last-observed time alone, which is exactly what we refuse.
		class := ClassUnresolvedNoBaseline
		if identity == "" {
			class = ClassUnresolvedNoIdentity
		}
		inc := 0
		if len(deduped) == 1 {
			inc = deduped[0].Count
		}
		findings = append(findings, AccountingFinding{
			Class: class, Identity: identity, Increment: inc,
			Detail: "only one observation for this identity; no reliable baseline (never attribute by last-observed time alone)",
		})
		return findings
	}

	perWindow := map[string]int{}
	// baseline is the last VALID temporal baseline observation. It is advanced for every
	// outcome except a late/out-of-order arrival, which must never become the baseline.
	baseline := deduped[0]
	for idx := 1; idx < len(deduped); idx++ {
		cur := deduped[idx]

		// Out-of-order / late arrival FIRST (this takes precedence over a counter reset): a
		// record observed before the current baseline is a late arrival. It must NOT become the
		// baseline, otherwise the next step would be measured from an earlier wall-clock time and
		// a stale count, fabricating an invalid span and delta. Retain the baseline. The zero
		// guards avoid treating a record with a missing observed time as "before" the baseline.
		if !baseline.Observed.IsZero() && !cur.Observed.IsZero() && cur.Observed.Before(baseline.Observed) {
			inc := cur.Count - baseline.Count
			if inc < 0 {
				inc = 0
			}
			findings = append(findings, AccountingFinding{
				Class: ClassUnresolvedMalformed, Identity: identity, Increment: inc,
				Detail: "observation time precedes the current baseline (out-of-order/late observation); baseline retained",
			})
			continue // retain baseline
		}

		// Reset: the cumulative counter dropped. Restart the baseline from this point; the step
		// itself is unresolved (we cannot know the real increment across a reset).
		if cur.Count < baseline.Count {
			findings = append(findings, AccountingFinding{
				Class: ClassUnresolvedMalformed, Identity: identity, Increment: 0,
				Detail: fmt.Sprintf("counter reset detected (%d -> %d); baseline restarted", baseline.Count, cur.Count),
			})
			baseline = cur
			continue
		}

		increment := cur.Count - baseline.Count
		if increment == 0 {
			baseline = cur // no new occurrences between observations; advance baseline
			continue
		}

		// The increment occurred somewhere in (baseline.Observed, cur.Observed]. We require the
		// whole span to be contained in a single eligible window; baseline is baseline.Count.
		spanFrom := baseline.Observed
		spanTo := cur.Observed
		if spanFrom.IsZero() || spanTo.IsZero() {
			findings = append(findings, AccountingFinding{
				Class: ClassUnresolvedNoBaseline, Identity: identity, Increment: increment,
				Detail: "missing observation timestamps; cannot bound the increment span",
			})
			baseline = cur
			continue
		}

		w, containedBy, overlapsAny := windowContaining(windows, spanFrom, spanTo)
		if containedBy != 1 {
			class := ClassUnresolvedSpanCrosses
			detail := "increment span is not fully contained in exactly one eligible window"
			if containedBy == 0 && !overlapsAny {
				class = ClassUnresolvedNoWindow
				detail = "increment span falls outside every eligible window"
			}
			findings = append(findings, AccountingFinding{
				Class: class, Identity: identity, Increment: increment,
				SpanFrom: spanFrom, SpanTo: spanTo,
				Detail: detail,
			})
			baseline = cur
			continue
		}
		if !w.IncrementsExpected {
			// Protected window: increments here are never attributed/forgiven.
			findings = append(findings, AccountingFinding{
				Class: ClassProtectedWindow, Identity: identity, Increment: increment,
				SpanFrom: spanFrom, SpanTo: spanTo, Window: w.Label,
				Detail: "increment contained in a protected (no-increment) window; not attributable/forgivable",
			})
			baseline = cur
			continue
		}

		// Reliable baseline + stable identity + fully-contained span in an increments-expected
		// window: this increment is attributable for accounting purposes.
		perWindow[w.Label] += increment
		detail := fmt.Sprintf("baseline=%d step=%d->%d", baseline.Count, baseline.Count, cur.Count)
		if cur.Note != "" {
			detail += " " + cur.Note
		}
		findings = append(findings, AccountingFinding{
			Class: ClassAttributed, Identity: identity, Increment: increment,
			SpanFrom: spanFrom, SpanTo: spanTo, Window: w.Label,
			Detail: detail,
		})
		baseline = cur
	}

	// Budget layer. Attribution does not imply approval. Without an established budget there
	// is no decision; the attributed increments remain Unresolved at the budget layer.
	if budgets == nil {
		return findings
	}
	for label, total := range perWindow {
		if budgets.MaxIncrementsPerWindow > 0 && total > budgets.MaxIncrementsPerWindow {
			findings = append(findings, AccountingFinding{
				Class: ClassUnresolvedBudgetExceeded, Identity: identity, Increment: total, Window: label,
				Detail: fmt.Sprintf("attributed %d increments exceed per-window budget %d", total, budgets.MaxIncrementsPerWindow),
			})
		}
	}
	return findings
}

// AccountByIdentity groups observations by Identity, runs the per-identity bounded accounting
// against the eligible windows, and aggregates a bounded report. Observations with an empty
// Identity are each accounted alone (they can never form a reliable step). Findings are
// returned in a deterministic order so bounded, capped renderings are reproducible.
func AccountByIdentity(observations []CounterObservation, windows []EligibleWindow, budgets *CounterBudgets) *AccountingReport {
	report := &AccountingReport{
		EligibleWindows:     len(windows),
		PerWindowAttributed: map[string]int{},
		BudgetsEstablished:  budgets != nil,
	}

	byID := map[string][]CounterObservation{}
	var noID []CounterObservation
	for _, o := range observations {
		if o.Identity == "" {
			noID = append(noID, o)
			continue
		}
		byID[o.Identity] = append(byID[o.Identity], o)
	}

	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var all []AccountingFinding
	for _, id := range ids {
		all = append(all, accountIdentityGroup(id, byID[id], windows, budgets)...)
	}
	for _, o := range noID {
		all = append(all, accountIdentityGroup("", []CounterObservation{o}, windows, budgets)...)
	}
	sort.SliceStable(all, func(a, b int) bool {
		if all[a].Window != all[b].Window {
			return all[a].Window < all[b].Window
		}
		if all[a].Class != all[b].Class {
			return all[a].Class < all[b].Class
		}
		if all[a].Identity != all[b].Identity {
			return all[a].Identity < all[b].Identity
		}
		return all[a].SpanFrom.Before(all[b].SpanFrom)
	})

	for _, f := range all {
		switch f.Class {
		case ClassAttributed:
			report.AttributedUnits++
			if f.Window != "" {
				report.PerWindowAttributed[f.Window] += f.Increment
			}
		case ClassProtectedWindow:
			report.ProtectedUnits++
		default:
			report.UnresolvedUnits++
		}
	}
	report.Findings = all
	return report
}
