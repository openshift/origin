package main

import (
	"io/ioutil"
	"os"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/alerts"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/pkg/synthetictests"
	"github.com/openshift/origin/pkg/synthetictests/allowedalerts"
	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

func newDevCommand() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "OpenShift Origin developer focused commands",
	}

	cmd.AddCommand(
		newRunAlertInvariantsCommand(),
		newRunDisruptionInvariantsCommand(),
		newUploadIntervalsCommand(),
	)
	return cmd
}

type alertInvariantOpts struct {
	intervalsFile string
	release       string
	fromRelease   string
	platform      string
	architecture  string
	network       string
	topology      string
}

func newRunAlertInvariantsCommand() *cobra.Command {
	opts := alertInvariantOpts{}

	cmd := &cobra.Command{
		Use:   "run-alert-invariants",
		Short: "Run alert invariant tests against an intervals file on disk",
		Long: templates.LongDesc(`
Run alert invariant tests against an e2e intervals json file from a CI run.
Requires the caller to specify the job variants as we do not query them live from
a running cluster.
`),

		RunE: func(cmd *cobra.Command, args []string) error {
			logrus.Info("running alert invariant tests")

			logrus.WithField("intervalsFile", opts.intervalsFile).Info("loading e2e intervals")
			intervals, err := readIntervalsFromFile(opts.intervalsFile)
			if err != nil {
				logrus.WithError(err).Fatal("error loading intervals file")
			}
			logrus.Infof("loaded %d intervals", len(intervals))

			jobType := &platformidentification.JobType{
				Release:      opts.release,
				FromRelease:  opts.fromRelease,
				Platform:     opts.platform,
				Architecture: opts.architecture,
				Network:      opts.network,
				Topology:     opts.topology,
			}

			logrus.Info("running tests")
			testCases := synthetictests.RunAlertTests(
				jobType,
				alerts.AllowedAlertsDuringUpgrade, // NOTE: may someway want a cli flag for conformance variant
				configv1.Default,
				allowedalerts.DefaultAllowances,
				intervals,
				&monitorapi.ResourcesMap{})
			for _, tc := range testCases {
				if tc.FailureOutput != nil {
					logrus.Warnf("FAIL: %s\n\n%s\n\n", tc.Name, tc.FailureOutput.Output)
				} else {
					logrus.Infof("PASS: %s", tc.Name)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.intervalsFile,
		"intervals-file", "e2e-events.json",
		"Path to an intervals file (i.e. e2e-events_20230214-203340.json). Can be obtained from a CI run in openshift-tests junit artifacts.")
	cmd.Flags().StringVar(
		&opts.platform,
		"platform", "gcp",
		"Platform for simulated cluster under test when intervals were gathered (aws, azure, gcp, metal, vsphere, etc)")
	cmd.Flags().StringVar(
		&opts.network,
		"network", "ocp",
		"Network plugin for simulated cluster under test when intervals were gathered")
	cmd.Flags().StringVar(
		&opts.release,
		"release", "4.13",
		"Release for simulated cluster under test when intervals were gathered")
	cmd.Flags().StringVar(
		&opts.fromRelease,
		"from-release", "4.13",
		"FromRelease simulated cluster under test was upgraded from when intervals were gathered (use \"\" for non-upgrade jobs, use matching value to --release for micro upgrades)")
	cmd.Flags().StringVar(
		&opts.architecture,
		"arch", "amd64",
		"Architecture for simulated cluster under test when intervals were gathered")
	cmd.Flags().StringVar(
		&opts.topology,
		"topology", "ha",
		"Topology for simulated cluster under test when intervals were gathered (ha, single)")
	return cmd
}

func readIntervalsFromFile(intervalsFile string) (monitorapi.Intervals, error) {
	jsonFile, err := os.Open(intervalsFile)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	jsonBytes, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	return monitorserialization.EventsFromJSON(jsonBytes)
}

func newRunDisruptionInvariantsCommand() *cobra.Command {
	// TODO: reusing alertInvariantOpts for now, seems we need the same for disruption.
	opts := alertInvariantOpts{}

	cmd := &cobra.Command{
		Use:   "run-disruption-invariants",
		Short: "Run disruption invariant tests against an intervals file on disk",
		Long: templates.LongDesc(`
Run disruption invariant tests against an e2e intervals json file from a CI run.
Requires the caller to specify the job variants as we do not query them live from
a running cluster.
`),

		RunE: func(cmd *cobra.Command, args []string) error {
			logrus.Info("running disruption invariant tests")

			logrus.WithField("intervalsFile", opts.intervalsFile).Info("loading e2e intervals")
			intervals, err := readIntervalsFromFile(opts.intervalsFile)
			if err != nil {
				logrus.WithError(err).Fatal("error loading intervals file")
			}
			logrus.Infof("loaded %d intervals", len(intervals))

			jobType := &platformidentification.JobType{
				Release:      opts.release,
				FromRelease:  opts.fromRelease,
				Platform:     opts.platform,
				Architecture: opts.architecture,
				Network:      opts.network,
				Topology:     opts.topology,
			}

			logrus.Info("running tests")

			// this isn't used for much, just the duration each test "ran":
			duration := 3 * time.Hour

			testCases := synthetictests.TestAllAPIBackendsForDisruption(intervals, duration, jobType)
			testCases = append(testCases, synthetictests.TestAllIngressBackendsForDisruption(intervals, duration, jobType)...)
			testCases = append(testCases, synthetictests.TestExternalBackendsForDisruption(intervals, duration, jobType)...)

			for _, tc := range testCases {
				if tc.FailureOutput != nil {
					logrus.Warnf("FAIL: %s\n\n%s\n\n", tc.Name, tc.FailureOutput.Output)
				} else {
					logrus.Infof("PASS: %s", tc.Name)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.intervalsFile,
		"intervals-file", "e2e-events.json",
		"Path to an intervals file (i.e. e2e-events_20230214-203340.json). Can be obtained from a CI run in openshift-tests junit artifacts.")
	cmd.Flags().StringVar(
		&opts.platform,
		"platform", "gcp",
		"Platform for simulated cluster under test when intervals were gathered (aws, azure, gcp, metal, vsphere, etc)")
	cmd.Flags().StringVar(
		&opts.network,
		"network", "ocp",
		"Network plugin for simulated cluster under test when intervals were gathered")
	cmd.Flags().StringVar(
		&opts.release,
		"release", "4.13",
		"Release for simulated cluster under test when intervals were gathered")
	cmd.Flags().StringVar(
		&opts.fromRelease,
		"from-release", "4.13",
		"FromRelease simulated cluster under test was upgraded from when intervals were gathered (use \"\" for non-upgrade jobs, use matching value to --release for micro upgrades)")
	cmd.Flags().StringVar(
		&opts.architecture,
		"arch", "amd64",
		"Architecture for simulated cluster under test when intervals were gathered")
	cmd.Flags().StringVar(
		&opts.topology,
		"topology", "ha",
		"Topology for simulated cluster under test when intervals were gathered (ha, single)")
	return cmd
}

type uploadIntervalsOpts struct {
	intervalsFile string
}

func newUploadIntervalsCommand() *cobra.Command {
	opts := uploadIntervalsOpts{}

	cmd := &cobra.Command{
		Use:   "upload-intervals",
		Short: "Upload an intervals file from a CI run to loki",

		RunE: func(cmd *cobra.Command, args []string) error {
			logrus.WithField("intervalsFile", opts.intervalsFile).Info("loading e2e intervals")
			intervals, err := readIntervalsFromFile(opts.intervalsFile)
			if err != nil {
				logrus.WithError(err).Fatal("error loading intervals file")
			}
			logrus.Infof("loaded %d intervals", len(intervals))

			err = monitor.UploadIntervalsToLoki(intervals)
			if err != nil {
				logrus.WithError(err).Fatal("error uploading intervals to loki")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.intervalsFile,
		"intervals-file", "e2e-events.json",
		"Path to an intervals file (i.e. e2e-events_20230214-203340.json). Can be obtained from a CI run in openshift-tests junit artifacts.")
	return cmd
}
