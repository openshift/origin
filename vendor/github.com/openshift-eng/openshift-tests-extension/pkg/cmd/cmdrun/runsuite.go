package cmdrun

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/openshift-eng/openshift-tests-extension/pkg/flags"
)

func NewRunSuiteCommand(registry *extension.Registry) *cobra.Command {
	opts := struct {
		componentFlags   *flags.ComponentFlags
		outputFlags      *flags.OutputFlags
		concurrencyFlags *flags.ConcurrencyFlags
		junitPath        string
		htmlPath         string
	}{
		componentFlags:   flags.NewComponentFlags(),
		outputFlags:      flags.NewOutputFlags(),
		concurrencyFlags: flags.NewConcurrencyFlags(),
		junitPath:        "",
		htmlPath:         "",
	}

	cmd := &cobra.Command{
		Use: "run-suite NAME",
		Short: "Run a group of tests by suite. This is more limited than origin, and intended for light local " +
			"development use. Orchestration parameters, scheduling, isolation, etc are not obeyed, and Ginkgo tests are executed serially.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancelCause := context.WithCancelCause(context.Background())
			defer cancelCause(errors.New("exiting"))

			abortCh := make(chan os.Signal, 2)
			go func() {
				<-abortCh
				fmt.Fprintf(os.Stderr, "Interrupted, terminating tests")
				cancelCause(errors.New("interrupt received"))

				select {
				case sig := <-abortCh:
					fmt.Fprintf(os.Stderr, "Interrupted twice, exiting (%s)", sig)
					switch sig {
					case syscall.SIGINT:
						os.Exit(130)
					default:
						os.Exit(130) // if we were interrupted, never return zero.
					}

				case <-time.After(30 * time.Minute): // allow time for cleanup.  If we finish before this, we'll exit
					fmt.Fprintf(os.Stderr, "Timed out during cleanup, exiting")
					os.Exit(130) // if we were interrupted, never return zero.
				}
			}()
			signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

			ext := registry.Get(opts.componentFlags.Component)
			if ext == nil {
				return fmt.Errorf("component not found: %s", opts.componentFlags.Component)
			}
			if len(args) != 1 {
				return fmt.Errorf("must specify one suite name")
			}
			suite, err := ext.GetSuite(args[0])
			if err != nil {
				return errors.Wrapf(err, "couldn't find suite: %s", args[0])
			}

			compositeWriter := extensiontests.NewCompositeResultWriter()
			defer func() {
				if err = compositeWriter.Flush(); err != nil {
					fmt.Fprintf(os.Stderr, "failed to write results: %v\n", err)
				}
			}()

			// JUnit writer if needed
			if opts.junitPath != "" {
				junitWriter, err := extensiontests.NewJUnitResultWriter(opts.junitPath, suite.Name)
				if err != nil {
					return errors.Wrap(err, "couldn't create junit writer")
				}
				compositeWriter.AddWriter(junitWriter)
			}
			// HTML writer if needed
			if opts.htmlPath != "" {
				htmlWriter, err := extensiontests.NewHTMLResultWriter(opts.htmlPath, suite.Name)
				if err != nil {
					return errors.Wrap(err, "couldn't create html writer")
				}
				compositeWriter.AddWriter(htmlWriter)
			}

			// JSON writer
			jsonWriter, err := extensiontests.NewJSONResultWriter(os.Stdout,
				extensiontests.ResultFormat(opts.outputFlags.Output))
			if err != nil {
				return err
			}
			compositeWriter.AddWriter(jsonWriter)

			specs, err := ext.GetSpecs().Filter(suite.Qualifiers)
			if err != nil {
				return errors.Wrap(err, "couldn't filter specs")
			}

			concurrency := opts.concurrencyFlags.MaxConcurency
			if suite.Parallelism > 0 {
				concurrency = min(concurrency, suite.Parallelism)
			}
			results, runErr := specs.Run(ctx, compositeWriter, concurrency)
			if opts.junitPath != "" {
				// we want to commit the results to disk regardless of the success or failure of the specs
				if err := writeResults(opts.junitPath, results); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to write test results to disk: %v\n", err)
				}
			}
			return runErr
		},
	}
	opts.componentFlags.BindFlags(cmd.Flags())
	opts.outputFlags.BindFlags(cmd.Flags())
	opts.concurrencyFlags.BindFlags(cmd.Flags())
	cmd.Flags().StringVarP(&opts.junitPath, "junit-path", "j", opts.junitPath, "write results to junit XML")
	cmd.Flags().StringVar(&opts.htmlPath, "html-path", opts.htmlPath, "write results to summary HTML")

	return cmd
}

func writeResults(jUnitPath string, results []*extensiontests.ExtensionTestResult) error {
	jUnitDir := filepath.Dir(jUnitPath)
	if err := os.MkdirAll(jUnitDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	encodedResults, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %v", err)
	}

	outputPath := filepath.Join(jUnitDir, fmt.Sprintf("extension_test_result_e2e_%s.json", time.Now().UTC().Format("20060102-150405")))
	return os.WriteFile(outputPath, encodedResults, 0644)
}
