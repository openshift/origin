package pathologicaleventlibrary

import (
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// authenticationPostTestGrace is how long AFTER an eligible AuthenticationComponentProxy
	// test ends its tolerance still applies; there is no pre-test grace. Before the tests waited for
	// the oauth-openshift rollout to complete, its events were observed 2-4 minutes after the test
	// ended. Now that they wait, the last DeploymentUpdated/ScalingReplicaSet occurrence lands 1-4
	// minutes before the test ends and the final OperatorStatusChanged (Progressing=False) only
	// seconds before, so the grace mainly absorbs Event timestamp second precision and recorder
	// latency, while still covering the previously observed lag should a rollout outlive its test.
	// It is a proximity allowance, not a claim the tests necessarily cause the rollouts.
	authenticationPostTestGrace = 5 * time.Minute

	// authenticationComponentProxyScalingReplicaSetRepeatLimit bounds cumulative ScalingReplicaSet
	// occurrences during the eligible authentication test windows and their post-test grace.
	// Observed cumulative counts up to 103 within eligible proxy-test windows, so the limit
	// provides bounded headroom while retaining a finite cap.
	authenticationComponentProxyScalingReplicaSetRepeatLimit = 120

	// authenticationComponentProxyExternalOIDCScalingReplicaSetRepeatLimit mirrors the oauth-openshift
	// cap for the openshift-oauth-apiserver operand. No live run has been observed yet, so the value
	// is unproven and may need retuning after the first full run.
	authenticationComponentProxyExternalOIDCScalingReplicaSetRepeatLimit = 120

	// authenticationOperatorDeploymentUpdatedRepeatLimit bounds cumulative DeploymentUpdated
	// occurrences on the authentication-operator. Grouping all the AuthenticationComponentProxy
	// tests together produced ~60 events, so the limit leaves ~15% headroom.
	authenticationOperatorDeploymentUpdatedRepeatLimit = 70

	// authenticationOperatorStatusChangedRepeatLimit bounds cumulative OperatorStatusChanged
	// occurrences on the authentication-operator. Observed 21 times as the proxy tests complete
	// rollouts, including cleanup.
	authenticationOperatorStatusChangedRepeatLimit = 30
)

// authenticationComponentProxyEligibleTestNames are the exact, full e2e test names of the
// AuthenticationComponentProxy serial suite. The hot-reload test (T3) is included
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
// of the AuthenticationComponentProxyExternalOIDC serial suite. The hot-reload test is included
// deliberately as an authorized eligible test even though it is not expected to roll the
// deployment. Matching is by exact full-name equality (never substring) so unrelated tests
// cannot widen the exception.
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

// authenticationEligibleTestNames is the union of both authentication proxy suites' eligible
// test names. All four authentication matchers draw their allowance windows from this one set,
// so each matcher's tolerance applies during either suite's windows.
func authenticationEligibleTestNames() sets.Set[string] {
	return sets.Set[string]{}.
		Union(authenticationComponentProxyEligibleTestNames()).
		Union(authenticationComponentProxyExternalOIDCEligibleTestNames())
}
