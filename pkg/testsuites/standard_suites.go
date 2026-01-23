package testsuites

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/origin/pkg/test/extensions"
	"github.com/openshift/origin/pkg/test/ginkgo"

	// these register framework.NewFrameworkExtensions responsible for
	// executing post-test actions, here debug and metrics gathering
	// see https://github.com/kubernetes/kubernetes/blob/v1.26.0/test/e2e/framework/framework.go#L175-L181
	// for more details
	_ "k8s.io/kubernetes/test/e2e/framework/debug/init"
	_ "k8s.io/kubernetes/test/e2e/framework/metrics/init"

	_ "github.com/openshift/origin/test/extended"
)

func InternalTestSuites() []*ginkgo.TestSuite {
	copied := make([]*ginkgo.TestSuite, 0, len(staticSuites))
	for i := range staticSuites {
		curr := staticSuites[i]
		copied = append(copied, &curr)
	}
	return copied
}

// AllTestSuites returns all test suites including internal suites and extension suites.
// It validates that no suite names are duplicated across internal and extension suites.
func AllTestSuites(ctx context.Context) ([]*ginkgo.TestSuite, error) {
	suites := InternalTestSuites()

	if len(os.Getenv("OPENSHIFT_SKIP_EXTERNAL_TESTS")) > 0 {
		logrus.Warning("Using built-in suites only due to OPENSHIFT_SKIP_EXTERNAL_TESTS being set")
		return suites, nil
	}

	// Create a map to track suite names and their sources for better error reporting
	suiteNameToSources := make(map[string][]string)
	for _, suite := range suites {
		suiteNameToSources[suite.Name] = []string{"internal"}
	}

	// Extract all test binaries from the release payload
	cleanup, binaries, err := extensions.ExtractAllTestBinaries(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to extract test binaries: %w", err)
	}
	defer cleanup()

	// Get info from all binaries
	extensionInfos, err := binaries.Info(ctx, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to get extension info: %w", err)
	}

	for _, e := range extensionInfos {
		for _, s := range e.Suites {
			extensionSource := fmt.Sprintf("extension %s:%s:%s", e.Component.Product, e.Component.Kind, e.Component.Name)
			if e.Source.SourceImage != "" {
				extensionSource = fmt.Sprintf("extension %s:%s:%s (image: %s)", e.Component.Product, e.Component.Kind, e.Component.Name, e.Source.SourceImage)
			}

			// Check if suite name conflicts with any existing suite name (internal or extension)
			if existingSources, exists := suiteNameToSources[s.Name]; exists {
				allSources := append(existingSources, extensionSource)
				return nil, fmt.Errorf("suite %q is declared by multiple sources: %v - there can be only one canonical source of a suite",
					s.Name, allSources)
			}

			// Add the suite name and its source to our tracking map
			suiteNameToSources[s.Name] = []string{extensionSource}

			var timeout time.Duration
			if s.TestTimeout != nil {
				timeout = *s.TestTimeout
			}

			suites = append(suites, &ginkgo.TestSuite{
				Name:                       s.Name,
				Description:                s.Description,
				Kind:                       ginkgo.KindExternal,
				Count:                      s.Count,
				Extension:                  e,
				Parallelism:                s.Parallelism,
				Qualifiers:                 s.Qualifiers,
				TestTimeout:                timeout,
				ClusterStabilityDuringTest: ginkgo.ClusterStabilityDuringTest(s.ClusterStability),
			})
		}
	}

	// Now handle setting qualifiers for parent suites once we've assembled the complete
	// list of suites.
	for _, e := range extensionInfos {
		for _, s := range e.Suites {
			for _, p := range s.Parents {
				for _, parent := range suites {
					if parent.Name == p {
						parent.Qualifiers = append(parent.Qualifiers, s.Qualifiers...)
					}
				}
			}
		}
	}

	return suites, nil
}

// staticSuites are all known test suites this binary should run
var staticSuites = []ginkgo.TestSuite{
	{
		Name: "openshift/conformance",
		Description: templates.LongDesc(`
		Tests that ensure an OpenShift cluster and components are working properly.
		`),
		Qualifiers: []string{
			withExcludedTestsFilter("name.contains('[Suite:openshift/conformance/')"),
		},
		Parallelism: 30,
	},
	{
		Name: "openshift/conformance/parallel",
		Description: templates.LongDesc(`
		Only the portion of the openshift/conformance test suite that run in parallel.
		`),
		Qualifiers: []string{
			withExcludedTestsFilter("name.contains('[Suite:openshift/conformance/parallel')"),
		},
		Parallelism:          30,
		MaximumAllowedFlakes: 15,
	},
	{
		Name: "openshift/conformance/serial",
		Description: templates.LongDesc(`
		Only the portion of the openshift/conformance test suite that run serially.
		`),
		Qualifiers: []string{
			// Standard early and late tests are included in the serial suite
			withExcludedTestsFilter(withStandardEarlyOrLateTests("name.contains('[Suite:openshift/conformance/serial')")),
		},
		TestTimeout: 40 * time.Minute,
		Parallelism: 1,
	},
	{
		Name: "openshift/disruptive",
		Description: templates.LongDesc(`
		The disruptive test suite.  Disruptive tests interrupt the cluster function such as by stopping/restarting the control plane or 
		changing the global cluster configuration in a way that can affect other tests.
		`),
		Qualifiers: []string{
			withStandardEarlyTests(`name.contains("[Feature:EtcdRecovery]") || name.contains("[Feature:NodeRecovery]")`),
		},
		// Duration of the quorum restore test exceeds 60 minutes.
		TestTimeout:                90 * time.Minute,
		ClusterStabilityDuringTest: ginkgo.Disruptive,
	},
	{
		Name: "openshift/build",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift build functionality.
		`),
		Qualifiers: []string{
			withStandardEarlyOrLateTests("name.contains('[Feature:Builds]')"),
		},
		Parallelism: 7,
		// TODO: Builds are really flaky right now, remove when we land perf updates and fix io on workers
		MaximumAllowedFlakes: 3,
		// Jenkins tests can take a really long time
		TestTimeout: 60 * time.Minute,
	},
	{
		Name: "openshift/templates",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift template functionality.
		`),
		Qualifiers: []string{
			withStandardEarlyOrLateTests("name.contains('[Feature:Templates]')"),
		},
		Parallelism: 1,
	},
	{
		Name: "openshift/image-registry",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift image-registry functionality.
		`),
		Qualifiers: []string{
			withStandardEarlyOrLateTests("name.contains('[sig-imageregistry]') && !name.contains('[Local]')"),
		},
	},
	{
		Name: "openshift/image-ecosystem",
		Description: templates.LongDesc(`
		Tests that exercise language and tooling images shipped as part of OpenShift.
		`),
		Qualifiers: []string{
			withStandardEarlyOrLateTests("name.contains('[Feature:ImageEcosystem]') && !name.contains('[Local]')"),
		},
		Parallelism: 7,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/jenkins-e2e",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift / Jenkins integrations provided by the OpenShift Jenkins image/plugins and the Pipeline Build Strategy.
		`),
		Qualifiers: []string{
			withStandardEarlyOrLateTests("name.contains('[Feature:Jenkins]')"),
		},
		Parallelism: 4,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/jenkins-e2e-rhel-only",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift / Jenkins integrations provided by the OpenShift Jenkins image/plugins and the Pipeline Build Strategy.
		`),
		Qualifiers: []string{
			withStandardEarlyOrLateTests("name.contains('[Feature:JenkinsRHELImagesOnly]')"),
		},
		Parallelism: 4,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/scalability",
		Description: templates.LongDesc(`
		Tests that verify the scalability characteristics of the cluster. Currently this is focused on core performance behaviors and preventing regressions.
		`),
		Qualifiers: []string{
			"name.contains('[Suite:openshift/scalability')",
		},
		Parallelism: 1,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/conformance-excluded",
		Description: templates.LongDesc(`
		Run only tests that are excluded from conformance. Makes identifying omitted tests easier.
		`),
		Qualifiers: []string{
			"!name.contains('[Suite:openshift/conformance/')",
		},
	},
	{
		Name: "openshift/test-cmd",
		Description: templates.LongDesc(`
		Run only tests for test-cmd.
		`),
		Qualifiers: []string{
			withStandardEarlyOrLateTests("name.contains('[Feature:LegacyCommandTests]')"),
		},
	},
	{
		Name: "openshift/csi",
		Description: templates.LongDesc(`
		Run tests for an CSI driver. The CSI driver tests are configured by two yaml file manifests.
		TEST_CSI_DRIVER_FILES environment variable must be a name of file with the upstream
		CSI driver test manifest. See
		https://github.com/openshift/kubernetes/blob/master/test/e2e/storage/external/README.md for
		required format of the file. Replace "master" with the OpenShift version you are testing
		against, e.g. "blob/release-4.17/test/..."
		TEST_OCP_CSI_DRIVER_FILES environment is optional and when set, must be a name of file
		with the OCP specific CSI driver test manifest. By specifying this file, the test suite will
		run the OCP specific tests in addition to the upstream tests. See
		https://github.com/openshift/origin/tree/main/test/extended/storage/csi for required format
		of the file. Replace "master" with the OpenShift version you are testing against, e.g.
		"blob/release-4.17/test/..."
		`),
		Qualifiers: []string{
			`name.contains("External Storage [Driver:") && !name.contains("[Disabled:") && !name.contains("[Flaky]") && !name.contains("[Disruptive]")`,
		},
	},
	{
		Name: "openshift/network/ipsec",
		Description: templates.LongDesc(`
		This test suite performs IPsec e2e tests covering control plane and data plane for east west and north south traffic scenarios.
		`),
		Qualifiers: []string{
			`name.contains("[Suite:openshift/network/ipsec")`,
		},
		Parallelism: 1,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/network/stress",
		Description: templates.LongDesc(`
		This test suite repeatedly verifies the networking function of the cluster in parallel to find flakes.
		`),
		Qualifiers: []string{
			withStandardEarlyOrLateTests(`!name.contains("[Feature:NetworkPolicy]") && !name.contains("[Serial:Self]") && name.contains("[Suite:openshift/conformance/") && name.contains("[sig-network]")`),
		},
		Parallelism: 60,
		Count:       12,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/network/live-migration",
		Description: templates.LongDesc(`
		This test suite performs CNI live migration either from SDN to OVN-Kubernetes or OVN-Kubernetes to SDN.
		`),
		Qualifiers: []string{
			`name.contains("[Suite:openshift/network/live-migration")`,
		},
		Count:                      1,
		TestTimeout:                4 * time.Hour,
		ClusterStabilityDuringTest: ginkgo.Stable,
	},
	{
		Name: "openshift/network/third-party",
		Description: templates.LongDesc(`
		The conformance testing suite for certified third-party CNI plugins.
		`),
		Qualifiers: []string{
			`name.contains("[Suite:k8s]") && name.contains("[sig-network]") && 
				(name.contains("[Conformance]") || 
					(name.contains("NetworkPolicy") && !name.contains("named port")) || 
					name.contains("[Feature:IPv6DualStack]"))`,
		},
	},
	{
		Name: "openshift/network/virtualization",
		Description: templates.LongDesc(`
		The conformance testing suite for virtualization related features.
		`),
		Qualifiers: []string{
			`name.contains("[Suite:openshift/network/virtualization")`,
		},
		Parallelism: 3,
	},
	{
		Name: "all",
		Description: templates.LongDesc(`
		Run all tests.
		`),
		Qualifiers: []string{
			`true`,
		},
	},
	{
		Name: "openshift/etcd/scaling",
		Description: templates.LongDesc(`
		This test suite runs vertical scaling tests to exercise the safe scale-up and scale-down of etcd members.
		`),
		Qualifiers: []string{
			withStandardEarlyOrLateTests(`name.contains("[Suite:openshift/etcd/scaling") || name.contains("[Feature:EtcdVerticalScaling]")`),
		},
		// etcd's vertical scaling test can take a while for apiserver rollouts to stabilize on the same revision
		TestTimeout: 60 * time.Minute,
	},
	{
		Name: "openshift/etcd/recovery",
		Description: templates.LongDesc(`
		This test suite runs etcd recovery tests to exercise the safe restore process of etcd members.
		`),
		Qualifiers: []string{
			withStandardEarlyOrLateTests(`name.contains("[Suite:openshift/etcd/recovery") || name.contains("[Feature:EtcdRecovery]")`),
		},
		// etcd's restore test can take a while for apiserver rollouts to stabilize
		Parallelism:                1,
		TestTimeout:                120 * time.Minute,
		ClusterStabilityDuringTest: ginkgo.Disruptive,
	},
	{
		Name: "openshift/etcd/certrotation",
		Description: templates.LongDesc(`
		This test suite runs etcd cert rotation tests to exercise the the automatic and manual certificate rotation.
		`),
		Qualifiers: []string{
			withStandardEarlyOrLateTests(`name.contains("[Suite:openshift/etcd/certrotation") || name.contains("[Feature:CertRotation]")`),
		},
		TestTimeout:                60 * time.Minute,
		Parallelism:                1,
		ClusterStabilityDuringTest: ginkgo.Stable,
	},
	{
		Name: "openshift/kube-apiserver/rollout",
		Description: templates.LongDesc(`
		This test suite runs kube-apiserver rollout reliability.
		`),
		Qualifiers: []string{
			withStandardEarlyOrLateTests(`name.contains("[Suite:openshift/kube-apiserver/rollout")`),
		},
		TestTimeout:                90 * time.Minute,
		Parallelism:                1,
		ClusterStabilityDuringTest: ginkgo.Stable,
	},
	{
		Name: "openshift/nodes/realtime",
		Description: templates.LongDesc(`
		This test suite runs tests to validate realtime functionality on nodes.
		`),
		Qualifiers: []string{
			`name.contains("[Suite:openshift/nodes/realtime")`,
		},
		TestTimeout: 30 * time.Minute,
	},
	{
		Name: "openshift/nodes/realtime/latency",
		Description: templates.LongDesc(`
		This test suite runs tests to validate realtime latency on nodes.
		`),
		Qualifiers: []string{
			`name.contains("[Suite:openshift/nodes/realtime/latency")`,
		},
		TestTimeout: 30 * time.Minute,
	},
	{
		Name: "openshift/usernamespace",
		Description: templates.LongDesc(`
		This test suite runs tests to validate user namespace functionality.
		`),
		Qualifiers: []string{
			`name.contains("[Suite:openshift/usernamespace")`,
		},
		TestTimeout: 60 * time.Minute,
	},
	{
		Name: "openshift/two-node",
		Description: templates.LongDesc(`
		This test suite runs tests to validate two-node.
		`),
		Qualifiers: []string{
			`name.contains("[Suite:openshift/two-node") || name.contains("[OCPFeatureGate:DualReplica]") || name.contains("[OCPFeatureGate:HighlyAvailableArbiter]")`,
		},
		TestTimeout: 60 * time.Minute,
	},
	{
		Name: "openshift/auth/external-oidc",
		Description: templates.LongDesc(`
		This test suite runs tests to validate cluster behavior when cluster authentication is configured to use an external OIDC provider.
		`),
		Qualifiers: []string{
			`name.contains("[Suite:openshift/auth/external-oidc") && !name.contains("[Skipped]")`,
		},
		TestTimeout:                120 * time.Minute,
		ClusterStabilityDuringTest: ginkgo.Disruptive,
	},
	{
		Name: "openshift/ote",
		Description: templates.LongDesc(`
		This test suite runs tests to validate OpenShift Test Extension features are working.
		`),
		Qualifiers: []string{
			`name.contains("[OTE]")`,
		},
		TestTimeout: 1 * time.Minute,
	},
	{
		Name: "openshift/test-image-stream-import-mode",
		Description: templates.LongDesc(`
		This test suite is to validate the ImageStreamImportMode feature.
		`),
		Qualifiers: []string{
			`name.contains("[Suite:openshift/test-image-stream-import-mode")`,
		},
		TestTimeout:                120 * time.Minute,
		ClusterStabilityDuringTest: ginkgo.Disruptive,
	},
	{
		Name: "openshift/disruptive-longrunning",
		Description: templates.LongDesc(`
		Long-running disruptive test suite. Tests in this suite are disruptive (cause node reboots,
		configuration changes, or cluster-wide disruptions) and take significant time to complete.
		Multiple teams can use this suite for their long-running disruptive tests.
		`),
		Qualifiers: []string{
			`name.contains("[Suite:openshift/disruptive-longrunning")`,
		},
		Parallelism:                1,
		TestTimeout:                40 * time.Minute,
		ClusterStabilityDuringTest: ginkgo.Disruptive,
	},
}

func withExcludedTestsFilter(baseExpr string) string {
	filter := ""
	for i, s := range extensions.ExcludedTests {
		if i > 0 {
			filter += " && "
		}
		filter += fmt.Sprintf("!name.contains('%s')", s)
	}

	if baseExpr != "" {
		return fmt.Sprintf("(%s) && (%s)", baseExpr, filter)
	}
	return filter
}
