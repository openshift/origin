package main

import (
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
	"github.com/spf13/pflag"
)

// TODO collapse this with cmd_runsuite
type RunUpgradeSuiteFlags struct {
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

func NewRunUpgradeSuiteFlags(streams genericclioptions.IOStreams, fromRepository string, availableSuites []*testginkgo.TestSuite) *RunUpgradeSuiteFlags {
	return &RunUpgradeSuiteFlags{
		GinkgoRunSuiteOptions: testginkgo.NewGinkgoRunSuiteOptions(streams),
		AvailableSuites:       availableSuites,

		FromRepository: fromRepository,
		IOStreams:      streams,
	}
}

func (f *RunUpgradeSuiteFlags) BindOptions(flags *pflag.FlagSet) {
	flags.StringVar(&f.FromRepository, "from-repository", f.FromRepository, "A container image repository to retrieve test images from.")
	flags.StringVar(&f.ProviderTypeOrJSON, "provider", f.ProviderTypeOrJSON, "The cluster infrastructure provider. Will automatically default to the correct value.")
	flags.StringVar(&f.ToImage, "to-image", f.ToImage, "Specify the image to test an upgrade to.")
	flags.StringSliceVar(&f.TestOptions, "options", f.TestOptions, "A set of KEY=VALUE options to control the test. See the help text.")
	f.GinkgoRunSuiteOptions.BindTestOptions(flags)
}

func (f *RunUpgradeSuiteFlags) ToOptions(args []string) (*RunUpgradeSuiteOptions, error) {
	// shallow copy to mutate
	ginkgoOptions := f.GinkgoRunSuiteOptions
	ginkgoOptions.SyntheticEventTests = pulledInvalidImages(f.FromRepository)
	// Upgrade test output is important for debugging because it shows linear progress
	// and when the CVO hangs.
	ginkgoOptions.IncludeSuccessOutput = true

	if len(f.ToImage) == 0 {
		return nil, fmt.Errorf("--to-image must be specified to run an upgrade test")
	}

	suite, err := f.GinkgoRunSuiteOptions.SelectSuite(f.AvailableSuites, args)
	if err != nil {
		return nil, err
	}

	o := &RunUpgradeSuiteOptions{
		GinkgoRunSuiteOptions: ginkgoOptions,
		Suite:                 suite,
		ToImage:               f.ToImage,
		FromRepository:        f.FromRepository,
		TestOptions:           f.TestOptions,
		IOStreams:             f.IOStreams,
	}

	return o, nil
}
