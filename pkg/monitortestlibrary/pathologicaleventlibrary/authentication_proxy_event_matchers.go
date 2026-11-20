package pathologicaleventlibrary

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// The allowance values for the AuthenticationComponentProxy and
// AuthenticationComponentProxyExternalOIDC gates: how long each gate's tolerance survives
// past the end of its eligible tests, and the repeat caps that bound each event kind's
// occurrences inside those windows.
const (
	// authenticationComponentProxyPostTestGrace is how long AFTER an eligible AuthenticationComponentProxy
	// test ends its tolerance still applies.
	authenticationComponentProxyPostTestGrace = 5 * time.Minute

	// authenticationComponentProxyExternalOIDCPostTestGrace is how long AFTER an eligible
	// AuthenticationComponentProxyExternalOIDC test ends its tolerance still applies.
	authenticationComponentProxyExternalOIDCPostTestGrace = 5 * time.Minute

	// authenticationComponentProxyDeploymentUpdatedRepeatLimit bounds the DeploymentUpdated
	// occurrences on the cross-gate authentication-operator deployment inside each gate's windows.
	// If all the authentication component proxy tests would be grouped together, the amount of
	// events are ~60. This amount it's ~15%. No live external-OIDC run observed yet; both gates
	// mirror the same bound until one is.
	authenticationComponentProxyDeploymentUpdatedRepeatLimit = 70

	// authenticationComponentProxyOperatorStatusChangedRepeatLimit bounds the
	// OperatorStatusChanged occurrences on the cross-gate authentication-operator deployment inside
	// each gate's windows. Observed 21 times as the integrated proxy tests complete rollouts,
	// including cleanup. No live external-OIDC run observed yet; both gates mirror the same
	// bound until one is.
	authenticationComponentProxyOperatorStatusChangedRepeatLimit = 30

	// authenticationComponentProxyScalingReplicaSetRepeatLimit bounds cumulative ScalingReplicaSet
	// occurrences during eligible AuthenticationComponentProxy windows and their post-test grace.
	// Observed cumulative counts up to 103 within eligible windows; the limit provides bounded
	// headroom while retaining a finite cap.
	authenticationComponentProxyScalingReplicaSetRepeatLimit = 120

	// authenticationComponentProxyExternalOIDCScalingReplicaSetRepeatLimit bounds cumulative
	// ScalingReplicaSet occurrences during eligible AuthenticationComponentProxyExternalOIDC
	// windows and their post-test grace. XXX: No live run observed yet.
	authenticationComponentProxyExternalOIDCScalingReplicaSetRepeatLimit = 40
)

// authenticationComponentProxyEligibleTestNames are the exact, full e2e test names whose runs authorize
// the scoped oauth-openshift ScalingReplicaSet tolerance. The hot-reload test (T3) is included
// deliberately as an authorized eligible test even though it is not expected to roll the
// deployment. Matching is by exact full-name equality (never substring) so unrelated tests
// cannot widen the exception.
func authenticationComponentProxyEligibleTestNames() sets.Set[string] {
	return sets.Set[string]{}.Insert(
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial] operator should validate OIDC IdP through component proxy with trustedCA`,
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial] operator should validate OIDC IdP through component proxy`,
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial] oauth-server/operator should hot-reload mounted CA file on change when spec.proxy.trustedCA is set`,
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial] oauth-server should perform full OIDC login flow through the proxy when auth proxy config is applied`,
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial] operator should fall back to original configuration on spec.proxy removal`,
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial] oauth-server should bypass proxy by directly connecting to idp to perform OIDC login flow when spec.proxy.noProxy contains idp`,
	)
}

// authenticationComponentProxyExternalOIDCEligibleTestNames are the exact, full e2e test names
// whose runs authorize the scoped openshift-oauth-apiserver ScalingReplicaSet tolerance and the
// cross-gate authentication-operator DeploymentUpdated and OperatorStatusChanged tolerance. The
// hot-reload test is included deliberately as an authorized eligible test even though it is not
// expected to roll the deployment. Matching is by exact full-name equality (never substring) so
// unrelated tests cannot widen the exception.
func authenticationComponentProxyExternalOIDCEligibleTestNames() sets.Set[string] {
	return sets.Set[string]{}.Insert(
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxyExternalOIDC][Skipped:MicroShift][Serial][Slow][Disruptive] operator should propagate proxy configuration to the oauth-apiserver operand`,
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxyExternalOIDC][Skipped:MicroShift][Serial][Slow][Disruptive] operator should propagate proxy configuration and trustedCA to the oauth-apiserver operand`,
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxyExternalOIDC][Skipped:MicroShift][Serial][Slow][Disruptive] operator should fall back to original configuration on spec.proxy removal`,
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxyExternalOIDC][Skipped:MicroShift][Serial][Slow][Disruptive] operator should go Degraded when spec.proxy.trustedCA references a missing ConfigMap`,
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxyExternalOIDC][Skipped:MicroShift][Serial][Slow][Disruptive] operator should re-sync trustedCA ConfigMap content to the oauth-apiserver operand when the source changes`,
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxyExternalOIDC][Skipped:MicroShift][Serial][Slow][Disruptive] oauth-apiserver should perform full OIDC login flow through the proxy when auth proxy config is applied`,
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxyExternalOIDC][Skipped:MicroShift][Serial][Slow][Disruptive] oauth-apiserver should perform full OIDC login flow through the HTTPS proxy with trustedCA when auth proxy config is applied`,
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxyExternalOIDC][Skipped:MicroShift][Serial][Slow][Disruptive] oauth-apiserver should perform full OIDC login flow when the proxy URL carries credentials`,
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxyExternalOIDC][Skipped:MicroShift][Serial][Slow][Disruptive] oauth-apiserver/operator should hot-reload mounted CA file on change when spec.proxy.trustedCA is set`,
		`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxyExternalOIDC][Skipped:MicroShift][Serial][Slow][Disruptive] oauth-apiserver should bypass proxy by directly connecting to idp to perform OIDC login flow when spec.proxy.noProxy contains idp`,
	)

}

// authenticationComponentProxyEventMatchers creates the pathological-event matchers for the
// AuthenticationComponentProxy and AuthenticationComponentProxyExternalOIDC serial tests,
// whose intentional operand rollouts push DeploymentUpdated, OperatorStatusChanged, and
// ScalingReplicaSet repeats past DuplicateEventThreshold. Matchers whose gates produced no
// eligible windows are omitted.
func authenticationComponentProxyEventMatchers(finalIntervals monitorapi.Intervals) []EventMatcher {
	integratedWindows := eligibleWindows(
		finalIntervals,
		authenticationComponentProxyEligibleTestNames(),
		authenticationComponentProxyPostTestGrace,
	)
	externalOIDCWindows := eligibleWindows(
		finalIntervals,
		authenticationComponentProxyExternalOIDCEligibleTestNames(),
		authenticationComponentProxyExternalOIDCPostTestGrace,
	)

	// DeploymentUpdated and OperatorStatusChanged sit on the authentication-operator deployment
	// both gates roll, so one Event object accumulates a single counter across both gates'
	// windows: CrossGateOverlapPathologicalEventMatcher owns them, enforcing each gate's cap
	// against that gate's own occurrences.
	crossEventMatchers := []crossEventMatcherSpec{
		{
			name: "AuthenticationComponentProxyDeploymentUpdated",
			cap:  authenticationComponentProxyDeploymentUpdatedRepeatLimit,
		},
		{
			name: "AuthenticationComponentProxyOperatorStatusChanged",
			cap:  authenticationComponentProxyOperatorStatusChangedRepeatLimit,
		},
	}

	// The ScalingReplicaSet locators are operand-specific deployments, so each gate's Event
	// objects and counters are its own: each matcher allows and discounts against its own
	// windows only.
	scalingMatchers := []scalingMatcherSpec{
		{
			gate:           "AuthenticationComponentProxy",
			windows:        integratedWindows,
			namespace:      "openshift-authentication",
			deployment:     "oauth-openshift",
			repeatOverride: authenticationComponentProxyScalingReplicaSetRepeatLimit,
		},
		{
			gate:           "AuthenticationComponentProxyExternalOIDC",
			windows:        externalOIDCWindows,
			namespace:      "openshift-oauth-apiserver",
			deployment:     "apiserver",
			repeatOverride: authenticationComponentProxyExternalOIDCScalingReplicaSetRepeatLimit,
		},
	}

	var matchers []EventMatcher
	for _, cross := range crossEventMatchers {
		gates := authenticationComponentProxyCrossEventGates(integratedWindows, externalOIDCWindows, cross.cap)
		if len(gates) == 0 {
			continue
		}
		matcher := &CrossGateOverlapPathologicalEventMatcher{
			delegate: &SimplePathologicalEventMatcher{
				name: cross.name,
				locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
					monitorapi.LocatorNamespaceKey:  regexp.MustCompile(`^openshift-authentication-operator$`),
					monitorapi.LocatorDeploymentKey: regexp.MustCompile(`^authentication-operator$`),
				},
				messageReasonRegex: regexp.MustCompile(fmt.Sprintf(`^%s$`, strings.TrimPrefix(cross.name, "AuthenticationComponentProxy"))),
			},
			gates:            gates,
			useLastTimestamp: true,
		}

		matcher.discountInWindowOccurrences(finalIntervals)
		matchers = append(matchers, matcher)
	}

	for _, scaling := range scalingMatchers {
		if len(scaling.windows) == 0 {
			continue
		}
		matcher := &OverlapOtherIntervalsPathologicalEventMatcher{
			delegate: &SimplePathologicalEventMatcher{
				name: scaling.gate + "ScalingReplicaSet",
				locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
					monitorapi.LocatorNamespaceKey:  regexp.MustCompile(fmt.Sprintf(`^%s$`, scaling.namespace)),
					monitorapi.LocatorDeploymentKey: regexp.MustCompile(fmt.Sprintf(`^%s$`, scaling.deployment)),
				},
				messageReasonRegex:      regexp.MustCompile(`^ScalingReplicaSet$`),
				repeatThresholdOverride: scaling.repeatOverride,
			},
			allowIfWithinIntervals: scaling.windows,
			useLastTimestamp:       true,
		}

		matcher.discountInWindowOccurrences(finalIntervals)
		matchers = append(matchers, matcher)
	}
	return matchers
}

// authenticationComponentProxyCrossEventGates builds the allowance list for a cross-gate
// authentication-operator event kind: one gate per suite that produced eligible windows,
// each bounded by cap inside its own windows.
func authenticationComponentProxyCrossEventGates(integratedWindows, externalOIDCWindows monitorapi.Intervals, cap int) []gateAllowance {
	var gates []gateAllowance
	if len(integratedWindows) > 0 {
		gates = append(gates, gateAllowance{
			allowIfWithinIntervals:  integratedWindows,
			repeatThresholdOverride: cap,
		})
	}
	if len(externalOIDCWindows) > 0 {
		gates = append(gates, gateAllowance{
			allowIfWithinIntervals:  externalOIDCWindows,
			repeatThresholdOverride: cap,
		})
	}
	return gates
}
