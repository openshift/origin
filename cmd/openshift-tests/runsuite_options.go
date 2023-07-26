package main

import (
	"fmt"
	"os"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/test/ginkgo"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
	"github.com/openshift/origin/test/e2e/upgrade"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

// TODO collapse this with cmd_runsuite
type RunSuiteOptions struct {
	GinkgoRunSuiteOptions *testginkgo.GinkgoRunSuiteOptions
	AvailableSuites       []*testginkgo.TestSuite

	FromRepository string
	Provider       string

	// Passed to the test process if set
	UpgradeSuite string
	ToImage      string
	TestOptions  []string

	// Shared by initialization code
	config *clusterdiscovery.ClusterConfiguration
}

func NewRunSuiteOptions(fromRepository string, availableSuites []*testginkgo.TestSuite) *RunSuiteOptions {
	return &RunSuiteOptions{
		GinkgoRunSuiteOptions: testginkgo.NewGinkgoRunSuiteOptions(os.Stdout, os.Stderr),
		AvailableSuites:       availableSuites,

		FromRepository: fromRepository,
	}
}

// SuiteWithInitializedProviderPreSuite loads the provider info, but does not
// exclude any tests specific to that provider.
func (o *RunSuiteOptions) SuiteWithInitializedProviderPreSuite() error {
	config, err := clusterdiscovery.DecodeProvider(o.Provider, o.GinkgoRunSuiteOptions.DryRun, true, nil)
	if err != nil {
		return err
	}
	o.config = config

	o.Provider = config.ToJSONString()
	return nil
}

// SuiteWithProviderPreSuite ensures that the suite filters out tests from providers
// that aren't relevant (see exutilcluster.ClusterConfig.MatchFn) by loading the
// provider info from the cluster or flags.
func (o *RunSuiteOptions) SuiteWithProviderPreSuite() error {
	if err := o.SuiteWithInitializedProviderPreSuite(); err != nil {
		return err
	}
	o.GinkgoRunSuiteOptions.MatchFn = o.config.MatchFn()
	return nil
}

// SuiteWithNoProviderPreSuite blocks out provider settings from being passed to
// child tests. Used with suites that should not have cloud specific behavior.
func (o *RunSuiteOptions) SuiteWithNoProviderPreSuite() error {
	o.Provider = `none`
	return o.SuiteWithProviderPreSuite()
}

// SuiteWithKubeTestInitializationPreSuite invokes the Kube suite in order to populate
// data from the environment for the CSI suite. Other suites should use
// suiteWithProviderPreSuite.
func (o *RunSuiteOptions) SuiteWithKubeTestInitializationPreSuite() error {
	if err := o.SuiteWithProviderPreSuite(); err != nil {
		return err
	}
	return clusterdiscovery.InitializeTestFramework(exutil.TestContext, o.config, o.GinkgoRunSuiteOptions.DryRun)
}

func (o *RunSuiteOptions) AsEnv() []string {
	var args []string
	args = append(args, "KUBE_TEST_REPO_LIST=") // explicitly prevent selective override
	args = append(args, fmt.Sprintf("KUBE_TEST_REPO=%s", o.FromRepository))
	args = append(args, fmt.Sprintf("TEST_PROVIDER=%s", o.Provider))
	args = append(args, fmt.Sprintf("TEST_JUNIT_DIR=%s", o.GinkgoRunSuiteOptions.JUnitDir))
	for i := 10; i > 0; i-- {
		if klog.V(klog.Level(i)).Enabled() {
			args = append(args, fmt.Sprintf("TEST_LOG_LEVEL=%d", i))
			break
		}
	}

	if len(o.UpgradeSuite) > 0 {
		upgradeOptions := UpgradeOptions{
			Suite:       o.UpgradeSuite,
			ToImage:     o.ToImage,
			TestOptions: o.TestOptions,
		}
		args = append(args, fmt.Sprintf("TEST_UPGRADE_OPTIONS=%s", upgradeOptions.ToEnv()))
	} else {
		args = append(args, "TEST_UPGRADE_OPTIONS=")
	}

	return args
}

func (o *RunSuiteOptions) SelectSuite(args []string) (*testginkgo.TestSuite, error) {
	suite, err := o.GinkgoRunSuiteOptions.SelectSuite(o.AvailableSuites, args)
	if err != nil {
		return nil, err
	}
	return suite, nil
}

func (o *RunSuiteOptions) BindOptions(flags *pflag.FlagSet) {
	flags.StringVar(&o.FromRepository, "from-repository", o.FromRepository, "A container image repository to retrieve test images from.")
	flags.StringVar(&o.Provider, "provider", o.Provider, "The cluster infrastructure provider. Will automatically default to the correct value.")
	o.GinkgoRunSuiteOptions.BindTestOptions(flags)
}

func (o *RunSuiteOptions) BindUpgradeOptions(flags *pflag.FlagSet) {
	flags.StringVar(&o.ToImage, "to-image", o.ToImage, "Specify the image to test an upgrade to.")
	flags.StringSliceVar(&o.TestOptions, "options", o.TestOptions, "A set of KEY=VALUE options to control the test. See the help text.")
}

// UpgradeTestPreSuite validates the test options and gathers data useful prior to launching the upgrade and it's
// related tests.
func (o *RunSuiteOptions) UpgradeTestPreSuite() error {
	if !o.GinkgoRunSuiteOptions.DryRun {
		testOpt := ginkgo.NewTestOptions(os.Stdout, os.Stderr)
		config, err := clusterdiscovery.DecodeProvider(os.Getenv("TEST_PROVIDER"), testOpt.DryRun, false, nil)
		if err != nil {
			return err
		}
		if err := clusterdiscovery.InitializeTestFramework(exutil.TestContext, config, testOpt.DryRun); err != nil {
			return err
		}
		klog.V(4).Infof("Loaded test configuration: %#v", exutil.TestContext)

		if err := upgrade.GatherPreUpgradeResourceCounts(); err != nil {
			return errors.Wrap(err, "error gathering preupgrade resource counts")
		}
	}

	// Upgrade test output is important for debugging because it shows linear progress
	// and when the CVO hangs.
	o.GinkgoRunSuiteOptions.IncludeSuccessOutput = true
	return SetUpgradeGlobalsFromTestOptions(o.TestOptions)
}
