package main

import (
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/spf13/pflag"
)

// TODO collapse this with cmd_runsuite
type RunSuiteFlags struct {
	GinkgoRunSuiteOptions *testginkgo.GinkgoRunSuiteOptions
	AvailableSuites       []*testginkgo.TestSuite

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

func NewRunSuiteFlags(streams genericclioptions.IOStreams, fromRepository string, availableSuites []*testginkgo.TestSuite) *RunSuiteFlags {
	return &RunSuiteFlags{
		GinkgoRunSuiteOptions: testginkgo.NewGinkgoRunSuiteOptions(streams),
		AvailableSuites:       availableSuites,

		FromRepository: fromRepository,
		IOStreams:      streams,
	}
}

// SuiteWithKubeTestInitializationPreSuite
// 1. invokes the Kube suite in order to populate data from the environment for the CSI suite (originally, but now everything).
// 2. ensures that the suite filters out tests from providers that aren't relevant (see exutilcluster.ClusterConfig.MatchFn) by
//    loading the provider info from the cluster or flags.
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

func (f *RunSuiteFlags) BindOptions(flags *pflag.FlagSet) {
	flags.StringVar(&f.FromRepository, "from-repository", f.FromRepository, "A container image repository to retrieve test images from.")
	flags.StringVar(&f.ProviderTypeOrJSON, "provider", f.ProviderTypeOrJSON, "The cluster infrastructure provider. Will automatically default to the correct value.")
	f.GinkgoRunSuiteOptions.BindTestOptions(flags)
}

func (f *RunSuiteFlags) ToOptions(args []string) (*RunSuiteOptions, error) {
	// shallow copy to mutate
	ginkgoOptions := f.GinkgoRunSuiteOptions

	suite, err := f.GinkgoRunSuiteOptions.SelectSuite(f.AvailableSuites, args)
	if err != nil {
		return nil, err
	}
	ginkgoOptions.SyntheticEventTests = pulledInvalidImages(f.FromRepository)

	providerConfig, err := f.SuiteWithKubeTestInitializationPreSuite()
	if err != nil {
		return nil, err
	}
	f.GinkgoRunSuiteOptions.MatchFn = providerConfig.MatchFn()

	o := &RunSuiteOptions{
		GinkgoRunSuiteOptions: ginkgoOptions,
		Suite:                 suite,
		FromRepository:        f.FromRepository,
		CloudProviderJSON:     providerConfig.ToJSONString(),
		IOStreams:             f.IOStreams,
	}

	return o, nil
}
