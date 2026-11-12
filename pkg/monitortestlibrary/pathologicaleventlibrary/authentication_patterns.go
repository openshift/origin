package pathologicaleventlibrary

import (
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// authenticationComponentProxyPostTestGrace is how long AFTER an eligible AuthenticationComponentProxy
	// test ends its tolerance still applies.
	authenticationComponentProxyPostTestGrace = 5 * time.Minute

	// authenticationComponentProxyExternalOIDCPostTestGrace is how long AFTER an eligible
	// AuthenticationComponentProxyExternalOIDC test ends its tolerance still applies.
	authenticationComponentProxyExternalOIDCPostTestGrace = 5 * time.Minute
)

const (
	// authenticationComponentProxyScalingReplicaSetRepeatLimit bounds cumulative ScalingReplicaSet
	// occurrences during eligible AuthenticationComponentProxy windows and their post-test grace.
	authenticationComponentProxyScalingReplicaSetRepeatLimit = 120

	// authenticationComponentProxyExternalOIDCScalingReplicaSetRepeatLimit bounds cumulative
	// ScalingReplicaSet occurrences during eligible AuthenticationComponentProxyExternalOIDC
	// windows and their post-test grace.
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
// shared authentication-operator DeploymentUpdated and OperatorStatusChanged tolerance. The
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
