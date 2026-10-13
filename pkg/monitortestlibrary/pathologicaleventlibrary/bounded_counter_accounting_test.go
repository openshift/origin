package pathologicaleventlibrary

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAccountByIdentity_GenericReusableNonOAuth exercises the domain-agnostic core through a
// second, non-OAuth domain to prove the abstraction is genuinely reusable and not welded to
// oauth. The "domain" here is an imagined controller-restart counter with its own window
// labels, identities, and a protected maintenance window — none of which the core knows about.
func TestAccountByIdentity_GenericReusableNonOAuth(t *testing.T) {
	base := time.Date(2026, 3, 4, 9, 0, 0, 0, time.UTC)

	// Two labelled windows plus a protected maintenance window, expressed purely with the
	// generic EligibleWindow type.
	windows := []EligibleWindow{
		{Label: "phase-a", IncrementsExpected: true, From: base, To: base.Add(5 * time.Minute)},
		{Label: "maintenance", IncrementsExpected: false, From: base.Add(10 * time.Minute), To: base.Add(15 * time.Minute)},
		{Label: "phase-b", IncrementsExpected: true, From: base.Add(20 * time.Minute), To: base.Add(25 * time.Minute)},
	}

	obsv := []CounterObservation{
		// identity "ctrl-1": a clean in-window step (2 -> 5) fully inside phase-a -> attributed 3.
		// The Note on the advancing observation is surfaced verbatim in the attributed detail.
		{Identity: "ctrl-1", Sequence: 1, HasSequence: true, Count: 2, Observed: base.Add(1 * time.Minute)},
		{Identity: "ctrl-1", Sequence: 2, HasSequence: true, Count: 5, Observed: base.Add(3 * time.Minute), Note: "restart batch"},
		// identity "ctrl-2": a step fully inside the protected maintenance window -> protected.
		{Identity: "ctrl-2", Sequence: 1, HasSequence: true, Count: 0, Observed: base.Add(11 * time.Minute)},
		{Identity: "ctrl-2", Sequence: 2, HasSequence: true, Count: 4, Observed: base.Add(13 * time.Minute)},
		// identity "ctrl-3": a step spanning phase-a into the gap -> unresolved (crosses).
		{Identity: "ctrl-3", Sequence: 1, HasSequence: true, Count: 1, Observed: base.Add(4 * time.Minute)},
		{Identity: "ctrl-3", Sequence: 2, HasSequence: true, Count: 3, Observed: base.Add(8 * time.Minute)},
	}

	// No budget established -> attribution reported, no decision.
	report := AccountByIdentity(obsv, windows, nil)

	assert.Equal(t, 3, report.EligibleWindows)
	assert.Equal(t, 1, report.AttributedUnits, "only ctrl-1's fully-contained step is attributed")
	assert.Equal(t, 3, report.PerWindowAttributed["phase-a"])
	assert.Equal(t, 1, report.ProtectedUnits, "ctrl-2's maintenance-window step is protected")
	assert.GreaterOrEqual(t, report.UnresolvedUnits, 1, "ctrl-3's crossing step is unresolved")
	assert.False(t, report.BudgetsEstablished)

	// The attributed finding carries the domain Note verbatim (opaque to the core).
	var attributed *AccountingFinding
	for i := range report.Findings {
		if report.Findings[i].Class == ClassAttributed {
			attributed = &report.Findings[i]
			break
		}
	}
	require.NotNil(t, attributed)
	assert.Equal(t, "phase-a", attributed.Window)
	assert.Contains(t, attributed.Detail, "restart batch", "domain Note must be surfaced in diagnostics")

	// With a per-window budget, the same attributed total is flagged as exceeded.
	budgeted := AccountByIdentity(obsv, windows, &CounterBudgets{MaxIncrementsPerWindow: 2})
	var budgetExceeded bool
	for _, f := range budgeted.Findings {
		if f.Class == ClassUnresolvedBudgetExceeded && f.Window == "phase-a" {
			budgetExceeded = true
		}
	}
	assert.True(t, budgetExceeded, "attributed 3 > per-window budget 2 must be flagged")
	assert.True(t, budgeted.BudgetsEstablished)
}

// TestAccountByIdentity_NoIdentityIsNeverPaired confirms the generic core never pairs
// observations that lack a stable identity into a step (each is accounted alone).
func TestAccountByIdentity_NoIdentityIsNeverPaired(t *testing.T) {
	base := time.Date(2026, 3, 4, 9, 0, 0, 0, time.UTC)
	windows := []EligibleWindow{{Label: "w", IncrementsExpected: true, From: base, To: base.Add(10 * time.Minute)}}
	obsv := []CounterObservation{
		{Identity: "", Count: 5, Observed: base.Add(1 * time.Minute)},
		{Identity: "", Count: 9, Observed: base.Add(2 * time.Minute)},
	}
	report := AccountByIdentity(obsv, windows, nil)
	assert.Equal(t, 0, report.AttributedUnits)
	assert.Equal(t, 2, report.UnresolvedUnits, "two no-identity observations stay two unresolved units, never one step")
}
