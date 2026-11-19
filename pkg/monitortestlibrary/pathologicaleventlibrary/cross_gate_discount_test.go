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

// TestAuthenticationComponentProxy_CrossGateDiscount pins that a gate's matcher does not
// charge the other gate's in-window occurrences as outside. The DeploymentUpdated and
// OperatorStatusChanged locators are the same authentication-operator deployment in both
// gates, so one Event object carries one cumulative counter across both gates' windows.
// The discount must therefore cover the union of the gates' windows, while the in-window
// allowance and its cap stay scoped to each gate's own windows.
func TestAuthenticationComponentProxy_CrossGateDiscount(t *testing.T) {
	integratedFrom := cpT0
	integratedTo := integratedFrom.Add(10 * time.Minute)
	externalFrom := integratedTo.Add(10 * time.Minute)
	externalTo := externalFrom.Add(10 * time.Minute)
	later := externalTo.Add(30 * time.Minute) // well past both gates' post-test grace

	tests := monitorapi.Intervals{
		cpTestInterval(cpEligibleFullNames[0], "Passed", integratedFrom, integratedTo),
		cpTestInterval(cpExternalOIDCFullName, "Passed", externalFrom, externalTo),
	}

	created := integratedFrom.Add(-time.Hour)
	du := deploymentUpdatedObject("du", created)

	// outsideOccurrence is the latest occurrence of the shared Event object, after every
	// window and grace ended.
	outsideOccurrence := func(series monitorapi.Intervals) monitorapi.Interval {
		return series[len(series)-1]
	}

	for _, tc := range []struct {
		name                 string
		occurrences          func() monitorapi.Intervals
		wantCredits          int
		wantIntegratedAllows bool
		wantExternalAllows   bool
	}{
		{
			name: "one occurrence after both gates' windows stays allowed",
			occurrences: func() monitorapi.Intervals {
				return concat(
					du.series(1, 25, integratedFrom.Add(time.Minute)),
					du.series(26, 50, externalFrom.Add(time.Minute)),
					du.series(51, 51, later))
			},
			wantCredits:          50,
			wantIntegratedAllows: true,
			wantExternalAllows:   true,
		},
		{
			name: "occurrences inside only the integrated gate's window stay allowed after it",
			occurrences: func() monitorapi.Intervals {
				return concat(
					du.series(1, 25, integratedFrom.Add(time.Minute)),
					du.series(26, 26, later))
			},
			wantCredits:          25,
			wantIntegratedAllows: true,
			wantExternalAllows:   true,
		},
		{
			name: "occurrences inside only the external gate's window stay allowed after it",
			occurrences: func() monitorapi.Intervals {
				return concat(
					du.series(1, 25, externalFrom.Add(time.Minute)),
					du.series(26, 26, later))
			},
			wantCredits:          25,
			wantIntegratedAllows: true,
			wantExternalAllows:   true,
		},
		{
			name: "occurrences outside every window still count against the limit",
			occurrences: func() monitorapi.Intervals {
				return concat(
					du.series(1, 25, integratedFrom.Add(time.Minute)),
					du.series(26, 50, externalFrom.Add(time.Minute)),
					du.series(51, 71, later))
			},
			wantCredits: 50,
		},
		{
			// The final observation's latest occurrence sits inside the external gate's window
			// with a cumulative count above the integrated cap. The integrated matcher must
			// allow it through the discount, not through its in-window path, so the discount
			// interval set must not widen the in-window allowance. The external matcher keeps
			// enforcing its own cap in its own window.
			name: "in-window allowance and cap stay scoped to each gate's own windows",
			occurrences: func() monitorapi.Intervals {
				return concat(
					du.series(1, 70, integratedFrom.Add(time.Minute)),
					du.series(71, 75, externalFrom.Add(time.Minute)))
			},
			wantCredits:          75,
			wantIntegratedAllows: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			series := tc.occurrences()
			outside := outsideOccurrence(series)
			t.Logf("outside observation: %s", cpEventDetails(outside, integratedFrom, externalTo))

			events := append(monitorapi.Intervals{}, tests...)
			events = append(events, series...)
			registry := NewUniversalPathologicalEventMatchers(nil, events)
			for _, expected := range []struct {
				name      string
				wantAllow bool
			}{
				{name: "AuthenticationComponentProxyDeploymentUpdated", wantAllow: tc.wantIntegratedAllows},
				{name: "AuthenticationComponentProxyExternalOIDCDeploymentUpdated", wantAllow: tc.wantExternalAllows},
			} {
				matcher, err := registry.GetMatcherByName(expected.name)
				require.NoError(t, err)
				replay := matcher.(*OverlapOtherIntervalsPathologicalEventMatcher)
				assert.Equalf(t, tc.wantCredits, replay.inWindowOccurrences[newEventObservation(outside)],
					"%s must discount the occurrences either gate's windows authorized", expected.name)
				assert.Equalf(t, expected.wantAllow, matcher.Allows(outside, v1.HighlyAvailableTopologyMode),
					"%s must allow the shared Event only when its residual outside count stays within the threshold", expected.name)
			}
		})
	}
}

func concat(parts ...monitorapi.Intervals) monitorapi.Intervals {
	var out monitorapi.Intervals
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}
