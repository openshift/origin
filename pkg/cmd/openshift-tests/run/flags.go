package run

import (
	"fmt"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"github.com/openshift/origin/pkg/clioptions/kubeconfig"
	"github.com/openshift/origin/pkg/clioptions/suiteselection"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
)

// TODO collapse this with cmd_runsuite
type RunSuiteFlags struct {
	GinkgoRunSuiteOptions   *testginkgo.GinkgoRunSuiteOptions
	TestSuiteSelectionFlags *suiteselection.TestSuiteSelectionFlags
	OutputFlags             *iooptions.OutputFlags
	AvailableSuites         []*testginkgo.TestSuite

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
		GinkgoRunSuiteOptions:   testginkgo.NewGinkgoRunSuiteOptions(streams),
		TestSuiteSelectionFlags: suiteselection.NewTestSuiteSelectionFlags(streams),
		OutputFlags:             iooptions.NewOutputOptions(),
		AvailableSuites:         availableSuites,

		FromRepository: fromRepository,
		IOStreams:      streams,
	}
}

// SuiteWithKubeTestInitializationPreSuite
//  1. invokes the Kube suite in order to populate data from the environment for the CSI suite (originally, but now everything).
//  2. ensures that the suite filters out tests from providers that aren't relevant (see exutilcluster.ClusterConfig.MatchFn) by
//     loading the provider info from the cluster or flags.
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
	flags.StringVar(&f.ProviderTypeOrJSON, "provider", f.ProviderTypeOrJSON, "The cluster infrastructure provider. Will automatically default to the correct value.")
	f.GinkgoRunSuiteOptions.BindFlags(flags)
	f.TestSuiteSelectionFlags.BindFlags(flags)
	f.OutputFlags.BindFlags(flags)
}

func (f *RunSuiteFlags) SetIOStreams(streams genericclioptions.IOStreams) {
	f.IOStreams = streams
	f.GinkgoRunSuiteOptions.SetIOStreams(streams)
}

func (f *RunSuiteFlags) ToOptions(args []string) (*RunSuiteOptions, error) {
	adminRESTConfig, err := kubeconfig.GetStaticRESTConfig()
	switch {
	case err != nil && f.GinkgoRunSuiteOptions.DryRun:
		fmt.Fprintf(f.ErrOut, "Unable to get admin rest config, skipping apigroup check in the dry-run mode: %v\n", err)
		adminRESTConfig = &rest.Config{}
	case err != nil && !f.GinkgoRunSuiteOptions.DryRun:
		return nil, fmt.Errorf("unable to get admin rest config, %w", err)
	}

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
		f.AvailableSuites,
		args,
		kubeconfig.NewDiscoveryGetter(adminRESTConfig),
		f.GinkgoRunSuiteOptions.DryRun,
		providerConfig.MatchFn(),
	)
	if err != nil {
		return nil, err
	}

	ginkgoOptions.FromRepository = f.FromRepository
	o := &RunSuiteOptions{
		GinkgoRunSuiteOptions: ginkgoOptions,
		Suite:                 suite,
		FromRepository:        f.FromRepository,
		CloudProviderJSON:     providerConfig.ToJSONString(),
		CloseFn:               closeFn,
		IOStreams:             f.IOStreams,
	}

	return o, nil
}
