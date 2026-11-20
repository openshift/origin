package pathologicaleventlibrary

import (
	"testing"
	"time"

	v1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

const cpExternalOIDCPrefix = `[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxyExternalOIDC][Skipped:MicroShift][Serial][Slow][Disruptive]`

var cpExternalOIDCFullName = cpExternalOIDCPrefix + " operator should propagate proxy configuration to the oauth-apiserver operand"

// TestAuthenticationComponentProxy_CrossGateDiscount pins the cross-gate evaluation of the
// DeploymentUpdated and OperatorStatusChanged events on the cross-gate authentication-operator
// deployment. One Event object carries one cumulative counter across both gates' windows, so
// one CrossGateOverlapPathologicalEventMatcher owns the whole evaluation: each gate's cap
// binds only the occurrences inside its own windows, the outside residual is computed against
// the merged union of the windows, and no gate's discount can re-admit what another gate's
// cap rejected.
func TestAuthenticationComponentProxy_CrossGateDiscount(t *testing.T) {
	integratedFrom := cpT0
	integratedTo := integratedFrom.Add(10 * time.Minute)
	externalFrom := integratedTo.Add(10 * time.Minute)
	externalTo := externalFrom.Add(10 * time.Minute)
	later := externalTo.Add(30 * time.Minute) // well past both gates' post-test grace

	// separatedTests yields two windows with a 5-minute gap between the integrated window's
	// grace end and the external window's start.
	separatedTests := func() monitorapi.Intervals {
		return monitorapi.Intervals{
			cpTestInterval(cpEligibleFullNames[0], "Passed", integratedFrom, integratedTo),
			cpTestInterval(cpExternalOIDCFullName, "Passed", externalFrom, externalTo),
		}
	}

	created := integratedFrom.Add(-time.Hour)
	du := deploymentUpdatedObject("du", created)

	// run replays the occurrences against a registry built from the given test intervals and
	// returns the cross-gate DeploymentUpdated matcher with its final observation.
	run := func(tests monitorapi.Intervals, series monitorapi.Intervals) (*CrossGateOverlapPathologicalEventMatcher, monitorapi.Interval) {
		events := append(monitorapi.Intervals{}, tests...)
		events = append(events, series...)
		registry := NewUniversalPathologicalEventMatchers(nil, events)
		matcher, err := registry.GetMatcherByName("AuthenticationComponentProxyDeploymentUpdated")
		require.NoError(t, err)
		return matcher.(*CrossGateOverlapPathologicalEventMatcher), series[len(series)-1]
	}

	for _, tc := range []struct {
		name        string
		tests       func() monitorapi.Intervals
		occurrences func() monitorapi.Intervals
		wantInUnion int
		wantPerGate []int
		wantAllows  bool
	}{
		{
			name:  "one occurrence after both gates' windows stays allowed",
			tests: separatedTests,
			occurrences: func() monitorapi.Intervals {
				return concat(
					du.series(1, 25, integratedFrom.Add(time.Minute)),
					du.series(26, 50, externalFrom.Add(time.Minute)),
					du.series(51, 51, later))
			},
			wantInUnion: 50,
			wantPerGate: []int{25, 25},
			wantAllows:  true,
		},
		{
			name:  "occurrences inside only the integrated gate's window stay allowed after it",
			tests: separatedTests,
			occurrences: func() monitorapi.Intervals {
				return concat(
					du.series(1, 25, integratedFrom.Add(time.Minute)),
					du.series(26, 26, later))
			},
			wantInUnion: 25,
			wantPerGate: []int{25, 0},
			wantAllows:  true,
		},
		{
			name:  "occurrences inside only the external gate's window stay allowed after it",
			tests: separatedTests,
			occurrences: func() monitorapi.Intervals {
				return concat(
					du.series(1, 25, externalFrom.Add(time.Minute)),
					du.series(26, 26, later))
			},
			wantInUnion: 25,
			wantPerGate: []int{0, 25},
			wantAllows:  true,
		},
		{
			name:  "occurrences outside every window still count against the limit",
			tests: separatedTests,
			occurrences: func() monitorapi.Intervals {
				return concat(
					du.series(1, 25, integratedFrom.Add(time.Minute)),
					du.series(26, 50, externalFrom.Add(time.Minute)),
					du.series(51, 71, later))
			},
			wantInUnion: 50,
			wantPerGate: []int{25, 25},
		},
		{
			// 71 occurrences fall entirely inside the integrated gate's window, past its cap
			// of 70. The owning gate's cap must hold: no other gate's discount may re-admit
			// the observation, and the outside residual of zero must not bypass it.
			name:  "over-cap occurrences inside one gate's window are rejected",
			tests: separatedTests,
			occurrences: func() monitorapi.Intervals {
				return concat(
					du.series(1, 71, integratedFrom.Add(time.Minute)),
					du.series(72, 72, later))
			},
			wantInUnion: 71,
			wantPerGate: []int{71, 0},
		},
		{
			// The external test starts before the integrated window's grace ends, so the two
			// windows overlap and the merged union covers the whole span. An increment whose
			// bounding observations fall in different gates' windows must keep its credits:
			// the union covers everything between them.
			name: "merged union retains credits for increments spanning both gates' windows",
			tests: func() monitorapi.Intervals {
				return monitorapi.Intervals{
					cpTestInterval(cpEligibleFullNames[0], "Passed", integratedFrom, integratedTo),
					cpTestInterval(cpExternalOIDCFullName, "Passed", integratedFrom.Add(10*time.Minute), integratedFrom.Add(20*time.Minute)),
				}
			},
			occurrences: func() monitorapi.Intervals {
				return concat(
					du.series(1, 1, integratedFrom.Add(9*time.Minute)),
					du.series(25, 25, integratedFrom.Add(16*time.Minute)),
					du.series(26, 26, integratedFrom.Add(40*time.Minute)))
			},
			wantInUnion: 25,
			wantPerGate: []int{25, 24},
			wantAllows:  true,
		},
		{
			// A 15-minute gap separates the two windows. An increment whose bounding
			// observations fall in different gates' windows spans the gap, so nothing between
			// them is provably inside the union and the increment keeps no credits.
			name: "gaps between windows stay uncovered",
			tests: func() monitorapi.Intervals {
				return monitorapi.Intervals{
					cpTestInterval(cpEligibleFullNames[0], "Passed", integratedFrom, integratedTo),
					cpTestInterval(cpExternalOIDCFullName, "Passed", integratedFrom.Add(30*time.Minute), integratedFrom.Add(40*time.Minute)),
				}
			},
			occurrences: func() monitorapi.Intervals {
				return concat(
					du.series(1, 1, integratedFrom.Add(5*time.Minute)),
					du.series(25, 25, integratedFrom.Add(35*time.Minute)),
					du.series(26, 26, integratedFrom.Add(60*time.Minute)))
			},
			wantInUnion: 1,
			wantPerGate: []int{1, 0},
		},
		{
			// 50 occurrences inside the integrated window, then a batched increment of 30
			// whose bounding observations fall in different gates' overlapping windows. The
			// increment is charged to both gates' caps, so the integrated gate's count of 80
			// exceeds its cap of 70 and the later outside observation is rejected even though
			// its residual count is one.
			name: "increments spanning both gates' windows are charged to both caps",
			tests: func() monitorapi.Intervals {
				return monitorapi.Intervals{
					cpTestInterval(cpEligibleFullNames[0], "Passed", integratedFrom, integratedTo),
					cpTestInterval(cpExternalOIDCFullName, "Passed", integratedFrom.Add(10*time.Minute), integratedFrom.Add(20*time.Minute)),
				}
			},
			occurrences: func() monitorapi.Intervals {
				return concat(
					du.series(1, 50, integratedFrom.Add(time.Minute)),
					du.series(80, 80, integratedFrom.Add(16*time.Minute)),
					du.series(81, 81, integratedFrom.Add(40*time.Minute)))
			},
			wantInUnion: 80,
			wantPerGate: []int{80, 30},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			series := tc.occurrences()
			outside := series[len(series)-1]
			t.Logf("final observation: %s", cpEventDetails(outside, integratedFrom, integratedTo))

			matcher, final := run(tc.tests(), series)
			counted := matcher.tally[newEventObservation(final)]
			assert.Equalf(t, tc.wantInUnion, counted.inUnionWindows,
				"the union of the gates' windows must cover exactly the provably in-window occurrences")
			assert.Equalf(t, tc.wantPerGate, counted.perGate,
				"each gate's count must hold only the occurrences attributed to that gate's windows")
			assert.Equalf(t, tc.wantAllows, matcher.Allows(final, v1.HighlyAvailableTopologyMode),
				"the cross-gate Event must be allowed only when every gate's cap and the outside residual hold")
		})
	}
}

// TestAuthenticationComponentProxy_CrossGateCapEnforcedThroughRegistry drives the whole
// registry: 71 DeploymentUpdated occurrences fall entirely inside the integrated gate's
// window, past its cap of 70. No other matcher may re-admit the observation, so
// AllowedByAny must reject it.
func TestAuthenticationComponentProxy_CrossGateCapEnforcedThroughRegistry(t *testing.T) {
	integratedFrom := cpT0
	integratedTo := integratedFrom.Add(10 * time.Minute)
	externalFrom := integratedTo.Add(10 * time.Minute)
	externalTo := externalFrom.Add(10 * time.Minute)

	created := integratedFrom.Add(-time.Hour)
	du := deploymentUpdatedObject("du", created)
	series := concat(
		du.series(1, 71, integratedFrom.Add(time.Minute)),
		du.series(72, 72, externalFrom.Add(time.Minute)))

	tests := monitorapi.Intervals{
		cpTestInterval(cpEligibleFullNames[0], "Passed", integratedFrom, integratedTo),
		cpTestInterval(cpExternalOIDCFullName, "Passed", externalFrom, externalTo),
	}
	events := append(monitorapi.Intervals{}, tests...)
	events = append(events, series...)

	registry := NewUniversalPathologicalEventMatchers(nil, events)
	overCap := series[len(series)-2] // count 71, latest occurrence inside the integrated window
	allowed, allower := registry.AllowedByAny(overCap, v1.HighlyAvailableTopologyMode)
	assert.Falsef(t, allowed, "an observation past its gate's cap must be rejected by the whole registry (allower=%v)", allower)
}

func concat(parts ...monitorapi.Intervals) monitorapi.Intervals {
	var out monitorapi.Intervals
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}
