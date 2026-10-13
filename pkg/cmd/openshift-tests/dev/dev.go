package dev

import (
	"io/ioutil"
	"os"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/alerts"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/pkg/monitortestlibrary/allowedalerts"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/monitortests/network/legacynetworkmonitortests"
	"github.com/openshift/origin/pkg/monitortests/testframework/legacytestframeworkmonitortests"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewDevCommand() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "OpenShift Origin developer focused commands",
	}

	cmd.AddCommand(
		newRunAlertInvariantsCommand(),
		newRunDisruptionInvariantsCommand(),
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
	o := alertInvariantOpts{}

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

			logrus.WithField("intervalsFile", o.intervalsFile).Info("loading e2e intervals")
			intervals, err := readIntervalsFromFile(o.intervalsFile)
			if err != nil {
				logrus.WithError(err).Fatal("error loading intervals file")
			}
			logrus.Infof("loaded %d intervals", len(intervals))

			jobType := &platformidentification.JobType{
				Release:      o.release,
				FromRelease:  o.fromRelease,
				Platform:     o.platform,
				Architecture: o.architecture,
				Network:      o.network,
				Topology:     o.topology,
			}

			logrus.Info("running tests")
			testCases := legacytestframeworkmonitortests.RunAlertTests(
				jobType,
				nil,
				alerts.AllowedAlertsDuringUpgrade, // NOTE: may someway want a cli flag for conformance variant
				configv1.Default,
				allowedalerts.DefaultAllowances,
				intervals,
				monitorapi.ResourcesMap{})
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
	cmd.Flags().StringVar(&o.intervalsFile,
		"intervals-file", "e2e-events.json",
		"Path to an intervals file (i.e. e2e-events_20230214-203340.json). Can be obtained from a CI run in openshift-tests junit artifacts.")
	cmd.Flags().StringVar(
		&o.platform,
		"platform", "gcp",
		"Platform for simulated cluster under test when intervals were gathered (aws, azure, gcp, metal, vsphere, etc)")
	cmd.Flags().StringVar(
		&o.network,
		"network", "ocp",
		"Network plugin for simulated cluster under test when intervals were gathered")
	cmd.Flags().StringVar(
		&o.release,
		"release", "4.13",
		"Release for simulated cluster under test when intervals were gathered")
	cmd.Flags().StringVar(
		&o.fromRelease,
		"from-release", "4.13",
		"FromRelease simulated cluster under test was upgraded from when intervals were gathered (use \"\" for non-upgrade jobs, use matching value to --release for micro upgrades)")
	cmd.Flags().StringVar(
		&o.architecture,
		"arch", "amd64",
		"Architecture for simulated cluster under test when intervals were gathered")
	cmd.Flags().StringVar(
		&o.topology,
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

	return monitorserialization.IntervalsFromJSON(jsonBytes)
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
			logrus.Info("running some disruption invariant tests (where possible)")

			logrus.WithField("intervalsFile", opts.intervalsFile).Info("loading e2e intervals")
			intervals, err := readIntervalsFromFile(opts.intervalsFile)
			if err != nil {
				logrus.WithError(err).Fatal("error loading intervals file")
			}
			logrus.Infof("loaded %d intervals", len(intervals))

			logrus.Info("running tests")
			junits := legacynetworkmonitortests.TestMultipleSingleSecondDisruptions(intervals, nil)
			for _, junit := range junits {
				if junit.FailureOutput != nil {
					logrus.Errorf("FAIL: %s", junit.Name)
					logrus.Error(junit.FailureOutput.Output)
				} else {
					logrus.Infof("PASS: %s", junit.Name)
				}
			}

			logrus.Warn("this command was nerfed, running only tests devs decide to include, not all disruption tests at this time")

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
