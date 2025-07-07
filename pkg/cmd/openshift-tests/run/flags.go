package run

import (
	"os"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"github.com/openshift/origin/pkg/clioptions/suiteselection"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
)

// TODO collapse this with cmd_runsuite
type RunSuiteFlags struct {
	GinkgoRunSuiteOptions   *testginkgo.GinkgoRunSuiteOptions
	TestSuiteSelectionFlags *suiteselection.TestSuiteSelectionFlags
	OutputFlags             *iooptions.OutputFlags

	FromRepository     string
	ProviderTypeOrJSON string

	// Passed to the test process if set
	UpgradeSuite string
	ToImage      string
	TestOptions  []string

	// Shared by initialization code
	config *clusterdiscovery.ClusterConfiguration

	genericclioptions.IOStreams
}

func NewRunSuiteFlags(streams genericclioptions.IOStreams, fromRepository string) *RunSuiteFlags {
	return &RunSuiteFlags{
		GinkgoRunSuiteOptions:   testginkgo.NewGinkgoRunSuiteOptions(streams),
		TestSuiteSelectionFlags: suiteselection.NewTestSuiteSelectionFlags(streams),
		OutputFlags:             iooptions.NewOutputOptions(),

		FromRepository: fromRepository,
		IOStreams:      streams,
	}
}

// SuiteWithKubeTestInitializationPreSuite
//  1. invokes the Kube suite in order to populate data from the environment for the CSI suite (originally, but now everything).
//  2. ensures that the suite filters out tests from providers that aren't relevant (see exutilcluster.ClusterConfig.MatchFn) by
//     loading the provider info from the cluster or flags, including API groups and feature gates.
func (f *RunSuiteFlags) SuiteWithKubeTestInitializationPreSuite() (*clusterdiscovery.ClusterConfiguration, error) {
	providerConfig, err := clusterdiscovery.DecodeProvider(f.ProviderTypeOrJSON, f.GinkgoRunSuiteOptions.DryRun, true, nil)
	if err != nil {
		return nil, err
	}

	if err := clusterdiscovery.InitializeTestFramework(exutil.TestContext, providerConfig, f.GinkgoRunSuiteOptions.DryRun); err != nil {
		return nil, err
	}

	return providerConfig, nil
}

func (f *RunSuiteFlags) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&f.FromRepository, "from-repository", f.FromRepository, "A container image repository to retrieve test images from.")
	flags.StringVar(&f.ProviderTypeOrJSON, "provider", os.Getenv("TEST_PROVIDER"), "The cluster infrastructure provider. Will automatically default to the correct value.")
	f.GinkgoRunSuiteOptions.BindFlags(flags)
	f.TestSuiteSelectionFlags.BindFlags(flags)
	f.OutputFlags.BindFlags(flags)
}

func (f *RunSuiteFlags) SetIOStreams(streams genericclioptions.IOStreams) {
	f.IOStreams = streams
	f.GinkgoRunSuiteOptions.SetIOStreams(streams)
}

func (f *RunSuiteFlags) ToOptions(args []string, availableSuites []*testginkgo.TestSuite, internalExtension *extension.Extension) (*RunSuiteOptions, error) {
	closeFn, err := f.OutputFlags.ConfigureIOStreams(f.IOStreams, f)
	if err != nil {
		return nil, err
	}

	// shallow copy to mutate
	ginkgoOptions := f.GinkgoRunSuiteOptions

	providerConfig, err := f.SuiteWithKubeTestInitializationPreSuite()
	if err != nil {
		return nil, err
	}
	suite, err := f.TestSuiteSelectionFlags.SelectSuite(
		availableSuites,
		args)
	if err != nil {
		return nil, err
	}

	o := &RunSuiteOptions{
		GinkgoRunSuiteOptions: ginkgoOptions,
		Suite:                 suite,
		Extension:             internalExtension,
		ClusterConfig:         providerConfig,
		FromRepository:        f.FromRepository,
		CloudProviderJSON:     providerConfig.ToJSONString(),
		CloseFn:               closeFn,
		IOStreams:             f.IOStreams,
	}

	return o, nil
}
