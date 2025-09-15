package cmdrun

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	"github.com/onsi/gomega"
	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/clioptions/imagesetup"
	"github.com/openshift/origin/pkg/clioptions/upgradeoptions"
	"github.com/openshift/origin/test/extended/util/image"
	"k8s.io/klog"

	ogenerated "github.com/openshift/origin/test/extended/util/annotate/generated"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	//	kgenerated "k8s.io/kubernetes/openshift-hack/e2e/annotate/generated"

	"github.com/openshift-eng/openshift-tests-extension/pkg/dbtime"
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/openshift-eng/openshift-tests-extension/pkg/flags"
	"github.com/openshift-eng/openshift-tests-extension/pkg/util/sets"

	exutil "github.com/openshift/origin/test/extended/util"
)

func NewRunTestCommand(registry *extension.Registry) *cobra.Command {
	opts := struct {
		componentFlags   *flags.ComponentFlags
		concurrencyFlags *flags.ConcurrencyFlags
		nameFlags        *flags.NamesFlags
		outputFlags      *flags.OutputFlags
		new bool
	}{
		componentFlags:   flags.NewComponentFlags(),
		nameFlags:        flags.NewNamesFlags(),
		outputFlags:      flags.NewOutputFlags(),
		concurrencyFlags: flags.NewConcurrencyFlags(),
	}

	cmd := &cobra.Command{
		Use:          "run-test [-n NAME...] [NAME]",
		Short:        "Runs tests by name",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ext := registry.Get(opts.componentFlags.Component)
			if ext == nil {
				return fmt.Errorf("component not found: %s", opts.componentFlags.Component)
			}
			if len(args) > 1 {
				return fmt.Errorf("use --names to specify more than one test")
			}
			opts.nameFlags.Names = append(opts.nameFlags.Names, args...)

			// allow reading tests from an stdin pipe
			info, err := os.Stdin.Stat()
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeCharDevice == 0 { // Check if input is from a pipe
				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					opts.nameFlags.Names = append(opts.nameFlags.Names, scanner.Text())
				}
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("error reading from stdin: %v", err)
				}
			}

			if len(opts.nameFlags.Names) == 0 {
				return fmt.Errorf("must specify at least one test")
			}

			/*
				specs, err := ext.FindSpecsByName(opts.nameFlags.Names...)
				if err != nil {
					return err
				}
			*/

			w, err := extensiontests.NewJSONResultWriter(os.Stdout, extensiontests.ResultFormat(opts.outputFlags.Output))
			if err != nil {
				return err
			}
			defer w.Flush()

			// return specs.Run(w, opts.concurrencyFlags.MaxConcurency)

			out, err := runSpecsForSuite(opts.nameFlags.Names...)
			if err != nil {
				return err
			}

			for _, o := range out {
				w.Write(o)
			}

			return nil
		},
	}
	opts.componentFlags.BindFlags(cmd.Flags())
	opts.nameFlags.BindFlags(cmd.Flags())
	opts.outputFlags.BindFlags(cmd.Flags())
	opts.concurrencyFlags.BindFlags(cmd.Flags())

	return cmd
}

func runSpecsForSuite(names ...string) (extensiontests.ExtensionTestResults, error) {
	// configure the ginkgo suite
	// --------------------------
	_, _, err := configureGinkgo()
	if err != nil {
		return nil, errors.Wrap(err, "configuring ginkgo")
	}
	// -------------------------

	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get current working directory")
	}

	specs := []types.TestSpec{}

	wantSet := sets.New(names...)

	ginkgo.GetSuite().WalkTests(func(testName string, test types.TestSpec) {
		for _, cl := range test.CodeLocations() {
			if strings.Contains(cl.String(), "/vendor/") {
				return
			}
		}

		annotatedTestName := annotateTestName(testName)

		if !wantSet.Has(annotatedTestName) && !wantSet.Has(testName) {
			return
		}

		specs = append(specs, test)
	})

	suiteConfig, reporterConfig, err := configureGinkgo()
	if err != nil {
		return nil, errors.Wrap(err, "configuring ginkgo")
	}

	// Before all setup
	// *****

	config, err := clusterdiscovery.DecodeProvider(os.Getenv("TEST_PROVIDER"), false, false, nil)
	if err != nil {
		panic(err)
	}
	if err := clusterdiscovery.InitializeTestFramework(exutil.TestContext, config, false); err != nil {
		panic(err)
	}
	klog.V(4).Infof("Loaded test configuration: %#v", exutil.TestContext)

	exutil.TestContext.ReportDir = os.Getenv("TEST_JUNIT_DIR")

	image.InitializeImages(os.Getenv("KUBE_TEST_REPO"))

	if err := imagesetup.VerifyImages(); err != nil {
		panic(err)
	}

	// Handle upgrade options
	upgradeOptionsYAML := os.Getenv("TEST_UPGRADE_OPTIONS")
	upgradeOptions, err := upgradeoptions.NewUpgradeOptionsFromYAML(upgradeOptionsYAML)
	if err != nil {
		panic(err)
	}

	if err := upgradeOptions.SetUpgradeGlobals(); err != nil {
		panic(err)
	}

	// *****

	_, _ = ginkgo.GetSuite().RunSpecs(
		specs,
		ginkgo.Labels{},
		"",
		cwd,
		ginkgo.GetFailer(),
		ginkgo.GetWriter(),
		*suiteConfig,
		*reporterConfig,
	)

	out := extensiontests.ExtensionTestResults{}
	for _, summary := range ginkgo.GetSuite().GetReport().SpecReports {
		if summary.NumAttempts == 0 {
			continue
		}

		result := extensiontests.ExtensionTestResult{
			Name:      annotateTestName(strings.Join(append(summary.ContainerHierarchyTexts, summary.LeafNodeText), " ")),
			Lifecycle: GetLifecycle(summary.Labels()),
			Duration:  int64(summary.RunTime),
			StartTime: (*dbtime.DBTime)(&summary.StartTime),
			EndTime:   (*dbtime.DBTime)(&summary.EndTime),
			Output:    summary.CapturedGinkgoWriterOutput,
			Error:     summary.CapturedStdOutErr,
		}

		switch {
		case summary.State == types.SpecStatePassed:
			result.Result = extensiontests.ResultPassed
		case summary.State == types.SpecStateSkipped:
			result.Result = extensiontests.ResultSkipped
			if len(summary.Failure.Message) > 0 {
				result.Output = fmt.Sprintf(
					"%s\n skip [%s:%d]: %s\n",
					result.Output,
					lastFilenameSegment(summary.Failure.Location.FileName),
					summary.Failure.Location.LineNumber,
					summary.Failure.Message,
				)
			} else if len(summary.Failure.ForwardedPanic) > 0 {
				result.Output = fmt.Sprintf(
					"%s\n skip [%s:%d]: %s\n",
					result.Output,
					lastFilenameSegment(summary.Failure.Location.FileName),
					summary.Failure.Location.LineNumber,
					summary.Failure.ForwardedPanic,
				)
			}
		case summary.State == types.SpecStateFailed, summary.State == types.SpecStatePanicked, summary.State == types.SpecStateInterrupted:
			result.Result = extensiontests.ResultFailed
			var errors []string
			if len(summary.Failure.ForwardedPanic) > 0 {
				if len(summary.Failure.Location.FullStackTrace) > 0 {
					errors = append(errors, fmt.Sprintf("\n%s\n", summary.Failure.Location.FullStackTrace))
				}
				errors = append(errors, fmt.Sprintf("fail [%s:%d]: Test Panicked: %s", lastFilenameSegment(summary.Failure.Location.FileName), summary.Failure.Location.LineNumber, summary.Failure.ForwardedPanic))
			}
			errors = append(errors, fmt.Sprintf("fail [%s:%d]: %s", lastFilenameSegment(summary.Failure.Location.FileName), summary.Failure.Location.LineNumber, summary.Failure.Message))
			result.Error = strings.Join(errors, "\n")
		default:
			panic(fmt.Sprintf("test produced unknown outcome: %#v", summary))
		}

		out = append(out, &result)
	}

	return out, nil
}

type NoopOutputInterceptor struct{}

func (interceptor NoopOutputInterceptor) StartInterceptingOutput()                      {}
func (interceptor NoopOutputInterceptor) StartInterceptingOutputAndForwardTo(io.Writer) {}
func (interceptor NoopOutputInterceptor) StopInterceptingAndReturnOutput() string       { return "" }
func (interceptor NoopOutputInterceptor) PauseIntercepting()                            {}
func (interceptor NoopOutputInterceptor) ResumeIntercepting()                           {}
func (interceptor NoopOutputInterceptor) Shutdown()                                     {}

func GetLifecycle(labels ginkgo.Labels) extensiontests.Lifecycle {
	for _, label := range labels {
		res := strings.Split(label, ":")
		if len(res) != 2 || !strings.EqualFold(res[0], "lifecycle") {
			continue
		}
		return MustLifecycle(res[1]) // this panics if unsupported lifecycle is used
	}

	return extensiontests.LifecycleBlocking
}

func MustLifecycle(l string) extensiontests.Lifecycle {
	switch extensiontests.Lifecycle(l) {
	case extensiontests.LifecycleInforming, extensiontests.LifecycleBlocking:
		return extensiontests.Lifecycle(l)
	default:
		panic(fmt.Sprintf("unknown test lifecycle: %s", l))
	}
}

func lastFilenameSegment(filename string) string {
	if parts := strings.Split(filename, "/vendor/"); len(parts) > 1 {
		return parts[len(parts)-1]
	}
	if parts := strings.Split(filename, "/src/"); len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return filename
}

func configureGinkgo() (*types.SuiteConfig, *types.ReporterConfig, error) {
	if !ginkgo.GetSuite().InPhaseBuildTree() {
		if err := ginkgo.GetSuite().BuildTree(); err != nil {
			return nil, nil, errors.Wrapf(err, "couldn't build ginkgo tree")
		}
	}

	// Ginkgo initialization
	ginkgo.GetSuite().ClearBeforeAndAfterSuiteNodes()
	suiteConfig, reporterConfig := ginkgo.GinkgoConfiguration()
	suiteConfig.RandomizeAllSpecs = true
	suiteConfig.Timeout = 24 * time.Hour
	reporterConfig.NoColor = true
	reporterConfig.Verbose = true
	ginkgo.SetReporterConfig(reporterConfig)

	// Write output to Stderr
	ginkgo.GinkgoWriter = ginkgo.NewWriter(os.Stderr)

	gomega.RegisterFailHandler(ginkgo.Fail)

	return &suiteConfig, &reporterConfig, nil
}

func annotateTestName(name string) string {
	annotation, ok := ogenerated.Annotations[name]
	if !ok {
		return name
	}

	return name + annotation
}
