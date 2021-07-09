package main

import (
	"strings"
	"time"

	"github.com/openshift/origin/pkg/synthetictests"
	"github.com/openshift/origin/pkg/test/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubectl/pkg/util/templates"

	_ "github.com/openshift/origin/test/extended"
	_ "github.com/openshift/origin/test/extended/util/annotate/generated"
)

func isDisabled(name string) bool {
	return strings.Contains(name, "[Disabled")
}

type testSuite struct {
	ginkgo.TestSuite

	PreSuite  func(opt *runOptions) error
	PostSuite func(opt *runOptions)

	PreTest func() error
}

type testSuites []testSuite

func (s testSuites) TestSuites() []*ginkgo.TestSuite {
	copied := make([]*ginkgo.TestSuite, 0, len(s))
	for i := range s {
		copied = append(copied, &s[i].TestSuite)
	}
	return copied
}

// staticSuites are all known test suites this binary should run
var staticSuites = testSuites{
	{
		TestSuite: ginkgo.TestSuite{
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
			Parallelism:         30,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
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
			SyntheticEventTests:  ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
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
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/disruptive",
			Description: templates.LongDesc(`
		The disruptive test suite.
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
			TestTimeout:         90 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.SystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
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
			Parallelism:         30,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
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
			TestTimeout:         60 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
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
			Parallelism:         1,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
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
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
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
			Parallelism:         7,
			TestTimeout:         20 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
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
			Parallelism:         4,
			TestTimeout:         20 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
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
			Parallelism:         4,
			TestTimeout:         20 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
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
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
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
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
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
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithNoProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "openshift/csi",
			Description: templates.LongDesc(`
		Run tests for an CSI driver. Set the TEST_CSI_DRIVER_FILES environment variable to the name of file with
		CSI driver test manifest. The manifest specifies Kubernetes + CSI features to test with the driver.
		See https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/external/README.md for required format of the file.
		`),
			Matches: func(name string) bool {
				if isDisabled(name) {
					return false
				}
				return strings.Contains(name, "External Storage [Driver:") && !strings.Contains(name, "[Disruptive]")
			},
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithKubeTestInitializationPreSuite,
		PostSuite: func(opt *runOptions) {
			printStorageCapabilities(opt.Out)
		},
	},
	{
		TestSuite: ginkgo.TestSuite{
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
				return (strings.Contains(name, "[Suite:openshift/conformance/") && strings.Contains(name, "[sig-network]")) || isStandardEarlyOrLateTest(name)
			},
			Parallelism:         60,
			Count:               12,
			TestTimeout:         20 * time.Minute,
			SyntheticEventTests: ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
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
		PreSuite: suiteWithProviderPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "experimental/reliability/minimal",
			Description: templates.LongDesc(`
		Set of highly reliable tests.
		`),
			Matches: func(name string) bool {

				_, exists := minimal[name]
				if !exists {
					return false
				}
				return !isDisabled(name) && strings.Contains(name, "[Suite:openshift/conformance")
			},
			Parallelism:          20,
			MaximumAllowedFlakes: 15,
			SyntheticEventTests:  ginkgo.JUnitForEventsFunc(synthetictests.StableSystemEventInvariants),
		},
		PreSuite: suiteWithKubeTestInitializationPreSuite,
	},
	{
		TestSuite: ginkgo.TestSuite{
			Name: "all",
			Description: templates.LongDesc(`
		Run all tests.
		`),
			Matches: func(name string) bool {
				return true
			},
		},
		PreSuite: suiteWithInitializedProviderPreSuite,
	},
}

// isStandardEarlyTest returns true if a test is considered part of the normal
// pre or post condition tests.
func isStandardEarlyTest(name string) bool {
	if !strings.Contains(name, "[Early]") {
		return false
	}
	return strings.Contains(name, "[Suite:openshift/conformance/parallel")
}

// isStandardEarlyOrLateTest returns true if a test is considered part of the normal
// pre or post condition tests.
func isStandardEarlyOrLateTest(name string) bool {
	if !strings.Contains(name, "[Early]") && !strings.Contains(name, "[Late]") {
		return false
	}
	return strings.Contains(name, "[Suite:openshift/conformance/parallel")
}

// suiteWithInitializedProviderPreSuite loads the provider info, but does not
// exclude any tests specific to that provider.
func suiteWithInitializedProviderPreSuite(opt *runOptions) error {
	config, err := decodeProvider(opt.Provider, opt.DryRun, true, nil)
	if err != nil {
		return err
	}
	opt.config = config

	opt.Provider = config.ToJSONString()
	return nil
}

// suiteWithProviderPreSuite ensures that the suite filters out tests from providers
// that aren't relevant (see exutilcluster.ClusterConfig.MatchFn) by loading the
// provider info from the cluster or flags.
func suiteWithProviderPreSuite(opt *runOptions) error {
	if err := suiteWithInitializedProviderPreSuite(opt); err != nil {
		return err
	}
	opt.MatchFn = opt.config.MatchFn()
	return nil
}

// suiteWithNoProviderPreSuite blocks out provider settings from being passed to
// child tests. Used with suites that should not have cloud specific behavior.
func suiteWithNoProviderPreSuite(opt *runOptions) error {
	opt.Provider = `none`
	return suiteWithProviderPreSuite(opt)
}

// suiteWithKubeTestInitialization invokes the Kube suite in order to populate
// data from the environment for the CSI suite. Other suites should use
// suiteWithProviderPreSuite.
func suiteWithKubeTestInitializationPreSuite(opt *runOptions) error {
	if err := suiteWithProviderPreSuite(opt); err != nil {
		return err
	}
	return initializeTestFramework(exutil.TestContext, opt.config, opt.DryRun)
}
