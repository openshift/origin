package test_report

import (
	"os"
	"path/filepath"
	"regexp"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/clioptions/imagesetup"
	"github.com/openshift/origin/pkg/cmd"
	origingenerated "github.com/openshift/origin/test/extended/util/annotate/generated"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	k8sgenerated "k8s.io/kubernetes/openshift-hack/e2e/annotate/generated"
	"sigs.k8s.io/yaml"
)

type RenderTestReportFlags struct {
	OutputDir string

	genericclioptions.IOStreams
}

func NewRenderTestReportOptions(streams genericclioptions.IOStreams, fromRepository string) *RenderTestReportFlags {
	return &RenderTestReportFlags{
		IOStreams: streams,
	}
}

func NewRenderTestReportCommand(streams genericclioptions.IOStreams) *cobra.Command {
	f := NewRenderTestReportOptions(streams, imagesetup.DefaultTestImageMirrorLocation)

	cmd := &cobra.Command{
		Use:              "test-report",
		Short:            "Write manifest indicating how many tests we have for each feature.",
		PersistentPreRun: cmd.NoPrintVersion,
		SilenceUsage:     true,
		SilenceErrors:    true,
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := f.ToOptions()
			if err != nil {
				return err
			}
			return o.Run()
		},
	}

	f.BindFlags(cmd.Flags())

	return cmd
}

func (f *RenderTestReportFlags) BindFlags(flags *pflag.FlagSet) {
	flags.StringVar(&f.OutputDir, "output-dir", f.OutputDir, "The directory where the rendered manifests are stored.")
}

func (f *RenderTestReportFlags) ToOptions() (*RenderTestReportOptions, error) {
	return &RenderTestReportOptions{
		OutputDir: f.OutputDir,
		IOStreams: f.IOStreams,
	}, nil
}

type RenderTestReportOptions struct {
	OutputDir string

	genericclioptions.IOStreams
}

// Run starts monitoring the cluster by invoking Start, periodically printing the
// events accumulated to Out. When the user hits CTRL+C or signals termination the
// condition intervals (all non-instantaneous events) are reported to Out.
func (o *RenderTestReportOptions) Run() error {
	featureGatesToTestNames := createFeatureGatesToTestNames()
	featureGateNames := sets.KeySet[string, sets.Set[string]](featureGatesToTestNames)

	testReporting := &configv1.TestReporting{
		TypeMeta: metav1.TypeMeta{
			Kind:       "TestReporting",
			APIVersion: "config.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}
	for _, featureGate := range sets.List(featureGateNames) {
		currFeatures := configv1.FeatureGateTests{
			FeatureGate: featureGate,
			Tests:       []configv1.TestDetails{},
		}

		testNames := featureGatesToTestNames[featureGate]
		for _, testName := range sets.List(testNames) {
			currFeatures.Tests = append(currFeatures.Tests,
				configv1.TestDetails{
					TestName: testName,
				},
			)
		}

		testReporting.Spec.TestsForFeatureGates = append(testReporting.Spec.TestsForFeatureGates, currFeatures)
	}

	outBytes, err := yaml.Marshal(testReporting)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(o.OutputDir, "test-reporting.yaml"), outBytes, 0644); err != nil {
		return err
	}

	return nil
}

func createFeatureGatesToTestNames() map[string]sets.Set[string] {
	featureGatesToTestNames := map[string]sets.Set[string]{}

	for testName := range origingenerated.Annotations {
		featureGates := featureGatesFromTestName(testName)
		if len(featureGates) == 0 {
			continue
		}
		for _, featureGate := range featureGates {
			testNames, ok := featureGatesToTestNames[featureGate]
			if !ok {
				testNames = sets.Set[string]{}
				featureGatesToTestNames[featureGate] = testNames
			}
			testNames.Insert(testName)
		}
	}
	for testName := range k8sgenerated.Annotations {
		featureGates := featureGatesFromTestName(testName)
		if len(featureGates) == 0 {
			continue
		}
		for _, featureGate := range featureGates {
			testNames, ok := featureGatesToTestNames[featureGate]
			if !ok {
				testNames = sets.Set[string]{}
				featureGatesToTestNames[featureGate] = testNames
			}
			testNames.Insert(testName)
		}
	}

	return featureGatesToTestNames
}

var (
	simpleOCPFeatureGateRegex = regexp.MustCompile(`\[OCPFeatureGate:(.+?)\]`)
	kubeFeatureGateRegex      = regexp.MustCompile(`\[FeatureGate:(.+?)\]`)
)

// remember that tests can have more than one featuregate specified
func featureGatesFromTestName(testName string) []string {
	featureGates := []string{}

	matches := simpleOCPFeatureGateRegex.FindAllStringSubmatch(testName, -1)
	for _, currMatch := range matches {
		if len(currMatch) > 1 {
			featureGates = append(featureGates, currMatch[1])
		}
	}

	matches = kubeFeatureGateRegex.FindAllStringSubmatch(testName, -1)
	for _, currMatch := range matches {
		if len(currMatch) > 1 {
			featureGates = append(featureGates, currMatch[1])
		}
	}
	return featureGates
}
