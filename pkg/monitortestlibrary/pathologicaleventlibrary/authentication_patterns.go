package pathologicaleventlibrary

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

// authenticationComponentProxyPostTestGrace is how long AFTER an eligible AuthenticationComponentProxy
// test ends the scoped oauth-openshift scaling tolerance still applies. The oauth rollout that
// a test drives can land slightly after the test interval closes; this bounded post-test
// window covers that without any pre-test grace. It is a proximity allowance, not a claim the
// tests necessarily cause the rollouts.
const authenticationComponentProxyPostTestGrace = 10 * time.Minute

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

// authenticationComponentProxyTestsDetected returns true if authentication
// component-proxy tests ran in this job. These tests intentionally trigger
// oauth-openshift rollouts that exceed the default duplicate-event threshold.
func authenticationComponentProxyTestsDetected(finalIntervals monitorapi.Intervals) bool {
	for _, eventInterval := range finalIntervals {
		if eventInterval.Source != monitorapi.SourceE2ETest {
			continue
		}
		if authenticationComponentProxyEligibleTestNames.Has(eventInterval.Locator.Keys[monitorapi.LocatorE2ETestKey]) {
			return true
		}
	}
	return false
}
