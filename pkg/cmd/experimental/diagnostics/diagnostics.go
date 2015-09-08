package diagnostics

import (
	"fmt"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"io"
	"os"
	"runtime/debug"

	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util"
	kutilerrors "k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/cmd/experimental/diagnostics/options"
	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// DiagnosticsOptions holds values received from command line flags as well as
// other objects generated for the command to operate.
type DiagnosticsOptions struct {
	// list of diagnostic names to limit what is run
	RequestedDiagnostics util.StringList
	// specify locations of host config files
	MasterConfigLocation string
	NodeConfigLocation   string
	// specify context name to be used for cluster-admin access
	ClientClusterContext string
	// indicate this is an openshift host despite lack of other indicators
	IsHost bool
	// We need a factory for creating clients. Creating a factory
	// creates flags as a byproduct, most of which we don't want.
	// The command creates these and binds only the flags we want.
	ClientFlags *flag.FlagSet
	Factory     *osclientcmd.Factory
	// LogOptions determine globally what the user wants to see and how.
	LogOptions *log.LoggerOptions
	// The Logger is built with the options and should be used for all diagnostic output.
	Logger *log.Logger
}

const longDescription = `
This utility helps troubleshoot and diagnose known problems. It runs
diagnostics using a client and/or the state of a running master /
node host.

    $ %[1]s

If run without flags, it will check for standard config files for
client, master, and node, and if found, use them for diagnostics.
You may also specify config files explicitly with flags, in which case
you will receive an error if they are not found. For example:

    $ %[1]s --master-config=/etc/openshift/master/master-config.yaml

* If master/node config files are not found and the --host flag is not
  present, host diagnostics are skipped.
* If the client has cluster-admin access, this access enables cluster
  diagnostics to run which regular users cannot.
* If a client config file is not found, client and cluster diagnostics
  are skipped.

NOTE: This is a beta version of diagnostics and may still evolve in a
different direction.
`

// NewCommandDiagnostics is the base command for running any diagnostics.
func NewCommandDiagnostics(name string, fullName string, out io.Writer) *cobra.Command {
	o := &DiagnosticsOptions{
		RequestedDiagnostics: util.StringList{},
		LogOptions:           &log.LoggerOptions{Out: out},
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "This utility helps you troubleshoot and diagnose.",
		Long:  fmt.Sprintf(longDescription, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete())

			failed, err, warnCount, errorCount := o.RunDiagnostics()
			o.Logger.Summary(warnCount, errorCount)

			kcmdutil.CheckErr(err)
			if failed {
				os.Exit(255)
			}

		},
	}
	cmd.SetOutput(out) // for output re: usage / help

	o.ClientFlags = flag.NewFlagSet("client", flag.ContinueOnError) // hide the extensive set of client flags
	o.Factory = osclientcmd.New(o.ClientFlags)                      // that would otherwise be added to this command
	cmd.Flags().AddFlag(o.ClientFlags.Lookup(config.OpenShiftConfigFlagName))
	cmd.Flags().AddFlag(o.ClientFlags.Lookup("context")) // TODO: find k8s constant
	cmd.Flags().StringVar(&o.ClientClusterContext, options.FlagClusterContextName, "", "client context to use for cluster administrator")
	cmd.Flags().StringVar(&o.MasterConfigLocation, options.FlagMasterConfigName, "", "path to master config file (implies --host)")
	cmd.Flags().StringVar(&o.NodeConfigLocation, options.FlagNodeConfigName, "", "path to node config file (implies --host)")
	cmd.Flags().BoolVar(&o.IsHost, options.FlagIsHostName, false, "look for systemd and journald units even without master/node config")
	flagtypes.GLog(cmd.Flags())
	options.BindLoggerOptionFlags(cmd.Flags(), o.LogOptions, options.RecommendedLoggerOptionFlags())
	options.BindDiagnosticFlag(cmd.Flags(), &o.RequestedDiagnostics, options.NewRecommendedDiagnosticFlag())

	return cmd
}

// Complete fills in DiagnosticsOptions needed if the command is actually invoked.
func (o *DiagnosticsOptions) Complete() error {
	var err error
	o.Logger, err = o.LogOptions.NewLogger()
	if err != nil {
		return err
	}

	return nil
}

// RunDiagnostics builds diagnostics based on the options and executes them, returning a summary.
func (o DiagnosticsOptions) RunDiagnostics() (bool, error, int, int) {
	failed := false
	warnings := []error{}
	errors := []error{}
	diagnostics := []types.Diagnostic{}
	AvailableDiagnostics := util.NewStringSet()
	AvailableDiagnostics.Insert(availableClientDiagnostics.List()...)
	AvailableDiagnostics.Insert(availableClusterDiagnostics.List()...)
	AvailableDiagnostics.Insert(availableHostDiagnostics.List()...)
	if len(o.RequestedDiagnostics) == 0 {
		o.RequestedDiagnostics = AvailableDiagnostics.List()
	} else if common := intersection(util.NewStringSet(o.RequestedDiagnostics...), AvailableDiagnostics); len(common) == 0 {
		o.Logger.Error("CED3012", log.EvalTemplate("CED3012", "None of the requested diagnostics are available:\n  {{.requested}}\nPlease try from the following:\n  {{.available}}",
			log.Hash{"requested": o.RequestedDiagnostics, "available": AvailableDiagnostics.List()}))
		return false, fmt.Errorf("No requested diagnostics available"), 0, 1
	} else if len(common) < len(o.RequestedDiagnostics) {
		errors = append(errors, fmt.Errorf("Not all requested diagnostics are available"))
		o.Logger.Error("CED3013", log.EvalTemplate("CED3013", `
Of the requested diagnostics:
    {{.requested}}
only these are available:
    {{.common}}
The list of all possible is:
    {{.available}}
		`, log.Hash{"requested": o.RequestedDiagnostics, "common": common.List(), "available": AvailableDiagnostics.List()}))
	}

	func() { // don't trust discovery/build of diagnostics; wrap panic nicely in case of developer error
		defer func() {
			if r := recover(); r != nil {
				failed = true
				stack := debug.Stack()
				errors = append(errors, fmt.Errorf("While building the diagnostics, a panic was encountered.\nThis is a bug in diagnostics. Error and stack trace follow: \n%v\n%s", r, stack))
			}
		}()
		detected, detectWarnings, detectErrors := o.detectClientConfig() // may log and return problems
		for _, warn := range detectWarnings {
			warnings = append(warnings, warn)
		}
		for _, err := range detectErrors {
			errors = append(errors, err)
		}
		if !detected { // there just plain isn't any client config file available
			o.Logger.Notice("CED3014", "No client configuration specified; skipping client and cluster diagnostics.")
		} else if rawConfig, err := o.buildRawConfig(); rawConfig == nil { // client config is totally broken - won't parse etc (problems may have been detected and logged)
			o.Logger.Error("CED3015", fmt.Sprintf("Client configuration failed to load; skipping client and cluster diagnostics due to error: %s", err.Error()))
			errors = append(errors, err)
		} else {
			if err != nil { // error encountered, proceed with caution
				o.Logger.Error("CED3016", fmt.Sprintf("Client configuration loading encountered an error, but proceeding anyway. Error was:\n%s", err.Error()))
				errors = append(errors, err)
			}
			clientDiags, ok, err := o.buildClientDiagnostics(rawConfig)
			failed = failed || !ok
			if ok {
				diagnostics = append(diagnostics, clientDiags...)
			}
			if err != nil {
				errors = append(errors, err)
			}

			clusterDiags, ok, err := o.buildClusterDiagnostics(rawConfig)
			failed = failed || !ok
			if ok {
				diagnostics = append(diagnostics, clusterDiags...)
			}
			if err != nil {
				errors = append(errors, err)
			}
		}

		hostDiags, ok, err := o.buildHostDiagnostics()
		failed = failed || !ok
		if ok {
			diagnostics = append(diagnostics, hostDiags...)
		}
		if err != nil {
			errors = append(errors, err)
		}
	}()

	if failed {
		return failed, kutilerrors.NewAggregate(errors), len(warnings), len(errors)
	}

	failed, err, numWarnings, numErrors := o.Run(diagnostics)
	numWarnings += len(warnings)
	numErrors += len(errors)
	return failed, err, numWarnings, numErrors
}

// Run performs the actual execution of diagnostics once they're built.
func (o DiagnosticsOptions) Run(diagnostics []types.Diagnostic) (bool, error, int, int) {
	warnCount := 0
	errorCount := 0
	for _, diagnostic := range diagnostics {
		func() { // wrap diagnostic panic nicely in case of developer error
			defer func() {
				if r := recover(); r != nil {
					errorCount += 1
					stack := debug.Stack()
					o.Logger.Error("CED3017",
						fmt.Sprintf("While running the %s diagnostic, a panic was encountered.\nThis is a bug in diagnostics. Error and stack trace follow: \n%s\n%s",
							diagnostic.Name(), fmt.Sprintf("%v", r), stack))
				}
			}()

			if canRun, reason := diagnostic.CanRun(); !canRun {
				if reason == nil {
					o.Logger.Notice("CED3018", fmt.Sprintf("Skipping diagnostic: %s\nDescription: %s", diagnostic.Name(), diagnostic.Description()))
				} else {
					o.Logger.Notice("CED3019", fmt.Sprintf("Skipping diagnostic: %s\nDescription: %s\nBecause: %s", diagnostic.Name(), diagnostic.Description(), reason.Error()))
				}
				return
			}

			o.Logger.Notice("CED3020", fmt.Sprintf("Running diagnostic: %s\nDescription: %s", diagnostic.Name(), diagnostic.Description()))
			r := diagnostic.Check()
			for _, entry := range r.Logs() {
				o.Logger.LogEntry(entry)
			}
			warnCount += len(r.Warnings())
			errorCount += len(r.Errors())
		}()
	}
	return errorCount > 0, nil, warnCount, errorCount
}

// TODO move upstream
func intersection(s1 util.StringSet, s2 util.StringSet) util.StringSet {
	result := util.NewStringSet()
	for key := range s1 {
		if s2.Has(key) {
			result.Insert(key)
		}
	}
	return result
}
