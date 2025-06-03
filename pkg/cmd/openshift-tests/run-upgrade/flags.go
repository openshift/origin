package run_upgrade

import (
	"fmt"

	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"github.com/openshift/origin/pkg/clioptions/suiteselection"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
)

// TODO collapse this with cmd_runsuite
type RunUpgradeSuiteFlags struct {
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

func NewRunUpgradeSuiteFlags(streams genericclioptions.IOStreams, fromRepository string, availableSuites []*testginkgo.TestSuite) *RunUpgradeSuiteFlags {
	return &RunUpgradeSuiteFlags{
		GinkgoRunSuiteOptions:   testginkgo.NewGinkgoRunSuiteOptions(streams),
		TestSuiteSelectionFlags: suiteselection.NewTestSuiteSelectionFlags(streams),
		OutputFlags:             iooptions.NewOutputOptions(),
		AvailableSuites:         availableSuites,

		FromRepository: fromRepository,
		IOStreams:      streams,
	}
}

func (f *RunUpgradeSuiteFlags) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&f.FromRepository, "from-repository", f.FromRepository, "A container image repository to retrieve test images from.")
	flags.StringVar(&f.ProviderTypeOrJSON, "provider", f.ProviderTypeOrJSON, "The cluster infrastructure provider. Will automatically default to the correct value.")
	flags.StringVar(&f.ToImage, "to-image", f.ToImage, "Specify the image to test an upgrade to.")
	flags.StringSliceVar(&f.TestOptions, "options", f.TestOptions, "A set of KEY=VALUE options to control the test. See the help text.")
	f.GinkgoRunSuiteOptions.BindFlags(flags)
	f.TestSuiteSelectionFlags.BindFlags(flags)
	f.OutputFlags.BindFlags(flags)
}

func (f *RunUpgradeSuiteFlags) SetIOStreams(streams genericclioptions.IOStreams) {
	f.IOStreams = streams
	f.GinkgoRunSuiteOptions.SetIOStreams(streams)
}

func (f *RunUpgradeSuiteFlags) ToOptions(args []string) (*RunUpgradeSuiteOptions, error) {
	closeFn, err := f.OutputFlags.ConfigureIOStreams(f.IOStreams, f)
	if err != nil {
		return nil, err
	}

	// shallow copy to mutate
	ginkgoOptions := f.GinkgoRunSuiteOptions
	// Upgrade test output is important for debugging because it shows linear progress
	// and when the CVO hangs.
	ginkgoOptions.IncludeSuccessOutput = true

	if len(f.ToImage) == 0 {
		return nil, fmt.Errorf("--to-image must be specified to run an upgrade test")
	}

	suite, err := f.TestSuiteSelectionFlags.SelectSuite(
		f.AvailableSuites,
		args)
	if err != nil {
		return nil, err
	}

	o := &RunUpgradeSuiteOptions{
		GinkgoRunSuiteOptions: ginkgoOptions,
		Suite:                 suite,
		ToImage:               f.ToImage,
		FromRepository:        f.FromRepository,
		TestOptions:           f.TestOptions,
		CloseFn:               closeFn,
		IOStreams:             f.IOStreams,
	}

	return o, nil
}
