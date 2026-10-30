package pathologicaleventlibrary

import (
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
)

// authenticationComponentProxyPostTestGrace is how long AFTER an eligible AuthenticationComponentProxy
// test ends its tolerance still applies; there is no pre-test grace. Before the tests waited for
// the oauth-openshift rollout to complete, its events were observed 2-4 minutes after the test
// ended. Now that they wait, the last DeploymentUpdated/ScalingReplicaSet occurrence lands 1-4
// minutes before the test ends and the final OperatorStatusChanged (Progressing=False) only
// seconds before, so the grace mainly absorbs Event timestamp second precision and recorder
// latency, while still covering the previously observed lag should a rollout outlive its test.
// It is a proximity allowance, not a claim the tests necessarily cause the rollouts.
const authenticationComponentProxyPostTestGrace = 5 * time.Minute

// authenticationComponentProxyEligibleTestNames are the exact, full e2e test names whose runs authorize
// the scoped oauth-openshift ScalingReplicaSet tolerance. The hot-reload test (T3) is included
// deliberately as an authorized eligible test even though it is not expected to roll the
// deployment. Matching is by exact full-name equality (never substring) so unrelated tests
// cannot widen the exception.
var authenticationComponentProxyEligibleTestNames = sets.Set[string]{}.Insert(
	`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial] operator should validate OIDC IdP through component proxy with trustedCA`,
	`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial] operator should validate OIDC IdP through component proxy`,
	`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial] oauth-server/operator should hot-reload mounted CA file on change when spec.proxy.trustedCA is set`,
	`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial] oauth-server should perform full OIDC login flow through the proxy when auth proxy config is applied`,
	`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial] operator should fall back to original configuration on spec.proxy removal`,
	`[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial] oauth-server should bypass proxy by directly connecting to idp to perform OIDC login flow when spec.proxy.noProxy contains idp`,
)
