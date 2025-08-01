package testsuites

import (
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo"
	"k8s.io/kubectl/pkg/util/templates"

	// these register framework.NewFrameworkExtensions responsible for
	// executing post-test actions, here debug and metrics gathering
	// see https://github.com/kubernetes/kubernetes/blob/v1.26.0/test/e2e/framework/framework.go#L175-L181
	// for more details
	_ "k8s.io/kubernetes/test/e2e/framework/debug/init"
	_ "k8s.io/kubernetes/test/e2e/framework/metrics/init"

	_ "github.com/openshift/origin/test/extended"
	_ "github.com/openshift/origin/test/extended/util/annotate/generated"
)

func StandardTestSuites() []*ginkgo.TestSuite {
	copied := make([]*ginkgo.TestSuite, 0, len(staticSuites))
	for i := range staticSuites {
		curr := staticSuites[i]
		copied = append(copied, &curr)
	}
	return copied
}

// staticSuites are all known test suites this binary should run
var staticSuites = []ginkgo.TestSuite{
	{
		Name: "openshift/conformance",
		Description: templates.LongDesc(`
		Tests that ensure an OpenShift cluster and components are working properly.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:openshift/conformance/")
		},
		Parallelism: 30,
	},
	{
		Name: "openshift/conformance/parallel",
		Description: templates.LongDesc(`
		Only the portion of the openshift/conformance test suite that run in parallel.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:openshift/conformance/parallel")
		},
		Parallelism:          30,
		MaximumAllowedFlakes: 15,
	},
	{
		Name: "openshift/conformance/serial",
		Description: templates.LongDesc(`
		Only the portion of the openshift/conformance test suite that run serially.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:openshift/conformance/serial") || isStandardEarlyOrLateTest(name)
		},
		TestTimeout: 40 * time.Minute,
	},
	{
		Name: "openshift/disruptive",
		Description: templates.LongDesc(`
		The disruptive test suite.  Disruptive tests interrupt the cluster function such as by stopping/restarting the control plane or 
		changing the global cluster configuration in a way that can affect other tests.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			// excluded due to stopped instance handling until https://bugzilla.redhat.com/show_bug.cgi?id=1905709 is fixed
			if strings.Contains(name, "Cluster should survive master and worker failure and recover with machine health checks") {
				return false
			}
			return strings.Contains(name, "[Feature:EtcdRecovery]") || strings.Contains(name, "[Feature:NodeRecovery]") || isStandardEarlyTest(name)

		},
		// Duration of the quorum restore test exceeds 60 minutes.
		TestTimeout:                90 * time.Minute,
		ClusterStabilityDuringTest: ginkgo.Disruptive,
	},
	{
		Name: "kubernetes/conformance",
		Description: templates.LongDesc(`
		The default Kubernetes conformance suite.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:k8s]") && strings.Contains(name, "[Conformance]")
		},
		Parallelism: 30,
	},
	{
		Name: "openshift/build",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift build functionality.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Feature:Builds]") || isStandardEarlyOrLateTest(name)
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
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Feature:Templates]") || isStandardEarlyOrLateTest(name)
		},
		Parallelism: 1,
	},
	{
		Name: "openshift/image-registry",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift image-registry functionality.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) || strings.Contains(name, "[Local]") {
				return false
			}
			return strings.Contains(name, "[sig-imageregistry]") || isStandardEarlyOrLateTest(name)
		},
	},
	{
		Name: "openshift/image-ecosystem",
		Description: templates.LongDesc(`
		Tests that exercise language and tooling images shipped as part of OpenShift.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) || strings.Contains(name, "[Local]") {
				return false
			}
			return strings.Contains(name, "[Feature:ImageEcosystem]") || isStandardEarlyOrLateTest(name)
		},
		Parallelism: 7,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/jenkins-e2e",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift / Jenkins integrations provided by the OpenShift Jenkins image/plugins and the Pipeline Build Strategy.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Feature:Jenkins]") || isStandardEarlyOrLateTest(name)
		},
		Parallelism: 4,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/jenkins-e2e-rhel-only",
		Description: templates.LongDesc(`
		Tests that exercise the OpenShift / Jenkins integrations provided by the OpenShift Jenkins image/plugins and the Pipeline Build Strategy.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Feature:JenkinsRHELImagesOnly]") || isStandardEarlyOrLateTest(name)
		},
		Parallelism: 4,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/scalability",
		Description: templates.LongDesc(`
		Tests that verify the scalability characteristics of the cluster. Currently this is focused on core performance behaviors and preventing regressions.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:openshift/scalability]")
		},
		Parallelism: 1,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/conformance-excluded",
		Description: templates.LongDesc(`
		Run only tests that are excluded from conformance. Makes identifying omitted tests easier.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return !strings.Contains(name, "[Suite:openshift/conformance/")
		},
	},
	{
		Name: "openshift/test-cmd",
		Description: templates.LongDesc(`
		Run only tests for test-cmd.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Feature:LegacyCommandTests]") || isStandardEarlyOrLateTest(name)
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
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "External Storage [Driver:") && !strings.Contains(name, "[Disruptive]")
		},
	},
	{
		Name: "openshift/network/ipsec",
		Description: templates.LongDesc(`
		This test suite performs IPsec e2e tests covering control plane and data plane for east west and north south traffic scenarios.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:openshift/network/ipsec")
		},
		Parallelism: 1,
		TestTimeout: 20 * time.Minute,
	},
	{
		Name: "openshift/network/stress",
		Description: templates.LongDesc(`
		This test suite repeatedly verifies the networking function of the cluster in parallel to find flakes.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			// Skip NetworkPolicy tests for https://bugzilla.redhat.com/show_bug.cgi?id=1980141
			if strings.Contains(name, "[Feature:NetworkPolicy]") {
				return false
			}
			// Serial:Self are tests that can't be run in parallel with a copy of itself
			if strings.Contains(name, "[Serial:Self]") {
				return false
			}
			return (strings.Contains(name, "[Suite:openshift/conformance/") && strings.Contains(name, "[sig-network]")) || isStandardEarlyOrLateTest(name)
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
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:openshift/network/live-migration")
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
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return inCNISuite(name)
		},
	},
	{
		Name: "openshift/network/virtualization",
		Description: templates.LongDesc(`
		The conformance testing suite for virtualization related features.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:openshift/network/virtualization")
		},
		Parallelism: 3,
	},
	{
		Name: "experimental/reliability/minimal",
		Description: templates.LongDesc(`
		Set of highly reliable tests.
		`),
		Matches: func(name string) bool {

			_, exists := minimal[name]
			if !exists {
				return false
			}
			return !isDisabled(name) && strings.Contains(name, "[Suite:openshift/conformance/parallel")
		},
		Parallelism:          20,
		MaximumAllowedFlakes: 15,
	},
	{
		Name: "all",
		Description: templates.LongDesc(`
		Run all tests.
		`),
		Matches: func(name string) bool {
			return true
		},
	},
	{
		Name: "openshift/etcd/scaling",
		Description: templates.LongDesc(`
		This test suite runs vertical scaling tests to exercise the safe scale-up and scale-down of etcd members.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:openshift/etcd/scaling") || strings.Contains(name, "[Feature:EtcdVerticalScaling]") || isStandardEarlyOrLateTest(name)
		},
		// etcd's vertical scaling test can take a while for apiserver rollouts to stabilize on the same revision
		TestTimeout: 60 * time.Minute,
	},
	{
		Name: "openshift/etcd/recovery",
		Description: templates.LongDesc(`
		This test suite runs etcd recovery tests to exercise the safe restore process of etcd members.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:openshift/etcd/recovery") || strings.Contains(name, "[Feature:EtcdRecovery]") || isStandardEarlyOrLateTest(name)
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
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:openshift/etcd/certrotation") || strings.Contains(name, "[Feature:CertRotation]") || isStandardEarlyOrLateTest(name)
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
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:openshift/kube-apiserver/rollout") || isStandardEarlyOrLateTest(name)
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
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:openshift/nodes/realtime")
		},
		TestTimeout: 30 * time.Minute,
	},
	{
		Name: "openshift/nodes/realtime/latency",
		Description: templates.LongDesc(`
		This test suite runs tests to validate realtime latency on nodes.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:openshift/nodes/realtime/latency")
		},
		TestTimeout: 30 * time.Minute,
	},
	{
		Name: "openshift/machine-config-operator/disruptive",
		Description: templates.LongDesc(`
		This test suite runs tests to validate machine-config-operator functionality.
		`),
		Matches: func(name string) bool {
			if isDisabled(name) {
				return false
			}
			return strings.Contains(name, "[Suite:openshift/machine-config-operator/disruptive")
		},
		TestTimeout:                120 * time.Minute,
		ClusterStabilityDuringTest: ginkgo.Disruptive,
	},
}
