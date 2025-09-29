package extensions

import (
	et "github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"k8s.io/apimachinery/pkg/util/sets"
)

// filterOutDisabledSpecs returns the specs with those that are disabled removed from the list
func filterOutDisabledSpecs(specs et.ExtensionTestSpecs) et.ExtensionTestSpecs {
	var disabledByReason = map[string][]string{
		// tests that require a local host
		"Local": {
			// Doesn't work on scaled up clusters
			"[Feature:ImagePrune]",
		},
		// tests that rely on special configuration that we do not yet support
		"SpecialConfig": {
			"[Feature:Audit]",      // Needs special configuration
			"[Feature:ImageQuota]", // Quota isn't turned on by default, we should do that and then reenable these tests
		},
		// tests that are known broken and need to be fixed upstream or in openshift
		// always add an issue here
		"Broken": {
			"should idle the service and DeploymentConfig properly",       // idling with a single service and DeploymentConfig
			"should answer endpoint and wildcard queries for the cluster", // currently not supported by dns operator https://github.com/openshift/cluster-dns-operator/issues/43

			// https://bugzilla.redhat.com/show_bug.cgi?id=1945091
			"[Feature:GenericEphemeralVolume]",

			// https://bugzilla.redhat.com/show_bug.cgi?id=1996128
			"[sig-network] [Feature:IPv6DualStack] should have ipv4 and ipv6 node podCIDRs",

			// https://bugzilla.redhat.com/show_bug.cgi?id=2004074
			"[sig-network-edge][Feature:Idling] Unidling [apigroup:apps.openshift.io][apigroup:route.openshift.io] should work with TCP (while idling)",
			"[sig-network-edge][Feature:Idling] Unidling with Deployments [apigroup:route.openshift.io] should work with TCP (while idling)",

			// https://bugzilla.redhat.com/show_bug.cgi?id=2070929
			"[sig-network][Feature:EgressIP][apigroup:operator.openshift.io] [internal-targets]",

			// https://issues.redhat.com/browse/OCPBUGS-967
			"[sig-network] IngressClass [Feature:Ingress] should prevent Ingress creation if more than 1 IngressClass marked as default",

			// https://issues.redhat.com/browse/OCPBUGS-3339
			"[sig-devex][Feature:ImageEcosystem][mysql][Slow] openshift mysql image Creating from a template should instantiate the template",
			"[sig-devex][Feature:ImageEcosystem][mariadb][Slow] openshift mariadb image Creating from a template should instantiate the template",

			// https://issues.redhat.com/browse/OCPBUGS-37799
			"[sig-builds][Feature:Builds][Slow] can use private repositories as build input build using an HTTP token should be able to clone source code via an HTTP token [apigroup:build.openshift.io]",
		},
		// tests that are known flaky
		"Flaky": {
			"openshift mongodb replication creating from a template", // flaking on deployment
		},
	}

	var disabledSpecs et.ExtensionTestSpecs
	for _, disabledList := range disabledByReason {
		var selectFunctions []et.SelectFunction
		for _, disabledName := range disabledList {
			selectFunctions = append(selectFunctions, et.NameContains(disabledName))
		}

		disabledSpecs = append(disabledSpecs, specs.SelectAny(selectFunctions)...)
	}

	disabledNames := sets.New[string]()
	for _, disabledSpec := range disabledSpecs {
		disabledNames.Insert(disabledSpec.Name)
	}

	enabledSpecs := specs[:0]
	for _, spec := range specs {
		if !disabledNames.Has(spec.Name) {
			enabledSpecs = append(enabledSpecs, spec)
		}
	}

	return enabledSpecs
}
