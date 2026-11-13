package testsuites

import (
	"fmt"
	"slices"
	"testing"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/openshift/origin/pkg/test/ginkgo"
)

// TestSuiteMonitorExclusions confines the legacy monitor exclusion to etcd recovery
// while preserving the existing exclusions for every other suite.
func TestSuiteMonitorExclusions(t *testing.T) {
	t.Parallel()
	foundRecovery := false
	for _, suite := range append(InternalTestSuites(), UpgradeTestSuites()...) {
		if suite.Name == "openshift/etcd/recovery" {
			foundRecovery = true
		}
		t.Run(suite.Name, func(t *testing.T) {
			t.Parallel()
			var expected []string
			switch suite.Name {
			case "openshift/etcd/recovery":
				expected = []string{"legacy-kube-apiserver-invariants"}
			case "openshift/etcd/scaling":
				expected = []string{"node-lifecycle"}
			}
			if !slices.Equal(suite.DisableMonitorTests, expected) {
				t.Errorf("expected disabled monitors %v, got %v", expected, suite.DisableMonitorTests)
			}
		})
	}
	if !foundRecovery {
		t.Fatal("openshift/etcd/recovery suite not found")
	}
}

// TestSuiteQualifiersValidCEL validates that all CEL expressions in suite qualifiers
// are syntactically valid
func TestSuiteQualifiersValidCEL(t *testing.T) {
	dummyTest := &extensiontests.ExtensionTestSpec{
		Name:   "[sig-test] Test a thing [Suite:openshift/conformance/parallel] [Early]",
		Source: "openshift:payload:origin",
	}
	dummySpecs := extensiontests.ExtensionTestSpecs{dummyTest}

	t.Run("standard suites", func(t *testing.T) {
		for _, suite := range staticSuites {
			t.Run(suite.Name, func(t *testing.T) {
				for i, qualifier := range suite.Qualifiers {
					t.Run(fmt.Sprintf("qualifier_%d", i), func(t *testing.T) {
						// Attempt to filter using the qualifier - this will validate the CEL expression
						_, err := dummySpecs.Filter([]string{qualifier})
						if err != nil {
							t.Errorf("Invalid CEL expression in suite %q, qualifier %d: %q\nError: %v",
								suite.Name, i, qualifier, err)
						}
					})
				}
			})
		}
	})

	t.Run("upgrade suites", func(t *testing.T) {
		for _, suite := range upgradeSuites {
			t.Run(suite.Name, func(t *testing.T) {
				for i, qualifier := range suite.Qualifiers {
					t.Run(fmt.Sprintf("qualifier_%d", i), func(t *testing.T) {
						// Attempt to filter using the qualifier - this will validate the CEL expression
						_, err := dummySpecs.Filter([]string{qualifier})
						if err != nil {
							t.Errorf("Invalid CEL expression in upgrade suite %q, qualifier %d: %q\nError: %v",
								suite.Name, i, qualifier, err)
						}
					})
				}
			})
		}
	})
}

func TestThirdPartySuiteMatchesHyperkubeTests(t *testing.T) {
	var thirdPartySuite *ginkgo.TestSuite
	for i := range staticSuites {
		if staticSuites[i].Name == "openshift/network/third-party" {
			thirdPartySuite = &staticSuites[i]
			break
		}
	}
	if thirdPartySuite == nil {
		t.Fatal("openshift/network/third-party suite not found")
	}

	candidateTests := extensiontests.ExtensionTestSpecs{
		{
			Name:   "[sig-network] NetworkPolicy API should support creating NetworkPolicy API operations [Conformance]",
			Source: "openshift:payload:hyperkube",
		},
		{
			Name:   "[sig-network] Networking should provide Internet connection for containers [Feature:Networking-IPv4] [Conformance]",
			Source: "openshift:payload:hyperkube",
		},
		{
			Name:   "[sig-network] NetworkPolicy should enforce policy based on Ports [Feature:IPv6DualStack]",
			Source: "openshift:payload:hyperkube",
		},
		{
			Name:   "[sig-network] NetworkPolicy named port should not match",
			Source: "openshift:payload:hyperkube",
		},
		{
			Name:   "[sig-storage] some storage test [Conformance]",
			Source: "openshift:payload:hyperkube",
		},
		{
			Name:   "[sig-network] some origin network test [Conformance]",
			Source: "openshift:payload:origin",
		},
	}

	filtered, err := candidateTests.Filter(thirdPartySuite.Qualifiers)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	expectedNames := map[string]bool{
		"[sig-network] NetworkPolicy API should support creating NetworkPolicy API operations [Conformance]":                 true,
		"[sig-network] Networking should provide Internet connection for containers [Feature:Networking-IPv4] [Conformance]": true,
		"[sig-network] NetworkPolicy should enforce policy based on Ports [Feature:IPv6DualStack]":                           true,
	}

	if len(filtered) != len(expectedNames) {
		t.Errorf("expected %d tests, got %d", len(expectedNames), len(filtered))
		for _, s := range filtered {
			t.Logf("  matched: %s", s.Name)
		}
	}
	for _, s := range filtered {
		if !expectedNames[s.Name] {
			t.Errorf("unexpected test matched: %s", s.Name)
		}
	}
}

func TestTNFMetricsSuiteSelectsOnlyTNFMetricsTests(t *testing.T) {
	var suite *ginkgo.TestSuite
	for i := range staticSuites {
		if staticSuites[i].Name == "openshift/two-node/tnf-metrics" {
			suite = &staticSuites[i]
			break
		}
	}
	if suite == nil {
		t.Fatal("openshift/two-node/tnf-metrics suite not found")
	}

	candidates := extensiontests.ExtensionTestSpecs{
		{Name: "[sig-etcd] [Suite:openshift/two-node] [TNFMetrics] cluster maintenance [Serial]"},
		{Name: "[sig-etcd] [Suite:openshift/two-node] [TNFMetrics] resource unmanage [Serial]"},
		{Name: "[sig-etcd] [Suite:openshift/two-node] [TNFMetrics] node maintenance [Serial]"},
		{Name: "[sig-etcd] [Suite:openshift/two-node] [TNFMetrics] fencing disable [Serial]"},
		{Name: "[sig-etcd] [Suite:openshift/two-node] existing TNF test [Serial]"},
		{Name: "[sig-etcd] [TNFMetrics] un-serialized metric test"},
		{Name: "[sig-etcd] [TNFMetrics] serial metric test for another topology [Serial]"},
	}

	filtered, err := candidates.Filter(suite.Qualifiers)
	if err != nil {
		t.Fatalf("filter TNF metrics suite: %v", err)
	}
	if len(filtered) != 4 {
		t.Fatalf("expected four serial TNF metrics tests, got %d: %#v", len(filtered), filtered)
	}
}

func TestTwoNodeSuiteExcludesDedicatedTNFMetricsTests(t *testing.T) {
	var suite *ginkgo.TestSuite
	for i := range staticSuites {
		if staticSuites[i].Name == "openshift/two-node" {
			suite = &staticSuites[i]
			break
		}
	}
	if suite == nil {
		t.Fatal("openshift/two-node suite not found")
	}

	candidates := extensiontests.ExtensionTestSpecs{
		{Name: "[sig-etcd] existing two-node recovery [Suite:openshift/two-node][Serial]"},
		{Name: "[sig-etcd] two-node feature gate [OCPFeatureGate:DualReplica][Serial]"},
		{Name: "[sig-etcd] TNF metrics [Suite:openshift/two-node][OCPFeatureGate:DualReplica][TNFMetrics][Serial]"},
	}
	filtered, err := candidates.Filter(suite.Qualifiers)
	if err != nil {
		t.Fatalf("filter two-node suite: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected two existing two-node tests and no TNF metrics test, got %d: %#v", len(filtered), filtered)
	}
	for _, test := range filtered {
		if test.Name == candidates[2].Name {
			t.Fatalf("dedicated TNF metrics test leaked into broad two-node suite: %s", test.Name)
		}
	}
}

func TestEtcdCertRotationSuiteSelection(t *testing.T) {
	t.Parallel()
	suites := InternalTestSuites()
	suite := findSuite(suites, "openshift/etcd/certrotation")
	if suite == nil {
		t.Fatal("openshift/etcd/certrotation suite not found")
	}
	leaderChanges := "[sig-etcd] etcd leader changes are not excessive [Late] [Suite:openshift/conformance/parallel]"
	for _, test := range []struct {
		name string
		want bool
	}{
		{name: leaderChanges},
		{name: leaderChanges + " [Feature:CertRotation]"},
		{name: leaderChanges + " [Suite:openshift/etcd/certrotation]"},
		{name: "[sig-etcd] cert rotation can manually rotate signer certificates [Feature:CertRotation]", want: true},
		{name: "[sig-etcd] cert rotation can manually rotate metrics signer certificates [Feature:CertRotation]", want: true},
		{name: "[sig-etcd] cert rotation can recreate dynamic certificates [Suite:openshift/etcd/certrotation]", want: true},
		{name: "[sig-etcd] cert rotation can recreate trust bundle [Suite:openshift/etcd/certrotation]", want: true},
		{name: "[sig-node] node checks [Early] [Suite:openshift/conformance/parallel]", want: true},
		{name: "[sig-node] node checks [Late] [Suite:openshift/conformance/parallel]", want: true},
		{name: "[sig-etcd] other leader changes coverage [Feature:CertRotation]"},
		{name: "[sig-etcd] other certificate coverage [Feature:CertRotation]", want: true},
		{name: "[sig-etcd] unrelated test"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidates := extensiontests.ExtensionTestSpecs{{Name: test.name, Source: "openshift:payload:origin"}}
			selected, err := candidates.Filter(suite.Qualifiers)
			if err != nil {
				t.Fatalf("failed to filter: %v", err)
			}
			if got := len(selected) == 1; got != test.want {
				t.Errorf("selected=%t, want %t", got, test.want)
			}
		})
	}
	for _, suiteName := range []string{"openshift/conformance/parallel", "openshift/etcd/recovery", "openshift/kube-apiserver/rollout"} {
		t.Run(suiteName, func(t *testing.T) {
			t.Parallel()
			otherSuite := findSuite(suites, suiteName)
			if otherSuite == nil {
				t.Fatalf("%s suite not found", suiteName)
			}
			candidates := extensiontests.ExtensionTestSpecs{{Name: leaderChanges, Source: "openshift:payload:origin"}}
			selected, err := candidates.Filter(otherSuite.Qualifiers)
			if err != nil {
				t.Fatalf("failed to filter: %v", err)
			}
			if len(selected) != 1 {
				t.Error("leader-changes invariant must remain selected outside certrotation")
			}
		})
	}
}

func TestEtcdScalingSuiteSelection(t *testing.T) {
	var suite *ginkgo.TestSuite
	for i := range staticSuites {
		if staticSuites[i].Name == "openshift/etcd/scaling" {
			suite = &staticSuites[i]
			break
		}
	}
	if suite == nil {
		t.Fatal("openshift/etcd/scaling suite not found")
	}

	candidates := extensiontests.ExtensionTestSpecs{
		{
			Name:   "[sig-etcd][Feature:EtcdVerticalScaling][Suite:openshift/etcd/scaling][Serial] etcd is able to vertically scale up and down with a single node [Timeout:60m][apigroup:machine.openshift.io]",
			Source: "openshift:payload:origin",
		},
		{
			Name:   "[sig-etcd][Feature:EtcdVerticalScaling][Suite:openshift/etcd/scaling][Serial] etcd is able to vertically scale up and down when CPMS is disabled [apigroup:machine.openshift.io]",
			Source: "openshift:payload:origin",
		},
		{
			Name:   "[sig-etcd][Feature:EtcdVerticalScaling][Suite:openshift/etcd/scaling][Serial] etcd is able to delete all masters with OnDelete strategy and wait for CPMSO to replace them [Timeout:120m][apigroup:machine.openshift.io]",
			Source: "openshift:payload:origin",
		},
		{
			Name:   "[sig-api-machinery][Feature:APIServer][Late] API LBs follow /readyz of kube-apiserver and stop sending requests [Suite:openshift/conformance/parallel]",
			Source: "openshift:payload:origin",
		},
		{
			Name:   "[sig-etcd] etcd leader changes are not excessive [Late] [Suite:openshift/conformance/parallel]",
			Source: "openshift:payload:origin",
		},
		{
			Name:   "[sig-node] Managed cluster should verify that nodes have no unexpected reboots [Late] [Suite:openshift/conformance/parallel]",
			Source: "openshift:payload:origin",
		},
	}

	filtered, err := candidates.Filter(suite.Qualifiers)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	expectedNames := map[string]bool{
		"[sig-etcd][Feature:EtcdVerticalScaling][Suite:openshift/etcd/scaling][Serial] etcd is able to delete all masters with OnDelete strategy and wait for CPMSO to replace them [Timeout:120m][apigroup:machine.openshift.io]": true,
		"[sig-etcd][Feature:EtcdVerticalScaling][Suite:openshift/etcd/scaling][Serial] etcd is able to vertically scale up and down with a single node [Timeout:60m][apigroup:machine.openshift.io]":                               true,
		"[sig-etcd][Feature:EtcdVerticalScaling][Suite:openshift/etcd/scaling][Serial] etcd is able to vertically scale up and down when CPMS is disabled [apigroup:machine.openshift.io]":                                         true,
		"[sig-node] Managed cluster should verify that nodes have no unexpected reboots [Late] [Suite:openshift/conformance/parallel]":                                                                                             true,
	}

	if len(filtered) != len(expectedNames) {
		t.Errorf("expected %d tests, got %d", len(expectedNames), len(filtered))
		for _, s := range filtered {
			t.Logf("  matched: %s", s.Name)
		}
	}
	for _, s := range filtered {
		if !expectedNames[s.Name] {
			t.Errorf("unexpected test matched: %s", s.Name)
		}
	}
}

func TestBGPVIPSuiteSelection(t *testing.T) {
	t.Parallel()
	const tags = `[sig-network][Jira:"Networking / On-Prem Load Balancer"][OCPFeatureGate:BGPBasedVIPManagement][Feature:BGPVIPManagement][apigroup:config.openshift.io][apigroup:machineconfiguration.openshift.io] BGP VIP management`
	nonDisruptive := []string{
		tags + " rendered [Suite:openshift/conformance/parallel]",
		tags + " frr-k8s [Suite:openshift/conformance/parallel]",
		tags + " sessions [Suite:openshift/conformance/parallel]",
		tags + " advertised [Suite:openshift/conformance/parallel]",
		tags + " ingress move [Serial][Requires:HypervisorSSHConfig] [Suite:openshift/conformance/serial]",
		tags + " traffic [Requires:HypervisorSSHConfig] [Suite:openshift/conformance/parallel]",
	}
	disruptive := []string{
		tags + " [Disruptive][Serial] failover [Requires:HypervisorSSHConfig]",
		tags + " [Disruptive][Serial] timers",
	}
	var candidates extensiontests.ExtensionTestSpecs
	for _, name := range append(slices.Clone(nonDisruptive), disruptive...) {
		candidates = append(candidates, &extensiontests.ExtensionTestSpec{Name: name, Source: "openshift:payload:origin"})
	}
	candidates = append(candidates,
		&extensiontests.ExtensionTestSpec{Name: "[sig-node] node checks [Early] [Suite:openshift/conformance/parallel]", Source: "openshift:payload:origin"},
		&extensiontests.ExtensionTestSpec{Name: nonDisruptive[0], Source: "openshift:payload:cluster-network-operator"},
		&extensiontests.ExtensionTestSpec{Name: disruptive[0], Source: "openshift:payload:cluster-network-operator"},
	)

	suites := InternalTestSuites()
	for suiteName, want := range map[string][]string{
		"openshift/bgp-vip":            nonDisruptive,
		"openshift/bgp-vip/disruptive": disruptive,
	} {
		t.Run(suiteName, func(t *testing.T) {
			t.Parallel()
			suite := findSuite(suites, suiteName)
			if suite == nil {
				t.Fatalf("%s suite not found", suiteName)
			}
			selected, err := candidates.Filter(suite.Qualifiers)
			if err != nil {
				t.Fatalf("failed to filter: %v", err)
			}
			var got []string
			for _, spec := range selected {
				if spec.Source != "openshift:payload:origin" {
					t.Errorf("selected a test from another binary: %s (%s)", spec.Name, spec.Source)
				}
				got = append(got, spec.Name)
			}
			if !slices.Equal(got, want) {
				t.Errorf("selected %q, want %q", got, want)
			}
		})
	}
}
