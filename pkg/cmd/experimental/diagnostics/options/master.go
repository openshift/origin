package options

import (
	"github.com/openshift/origin/pkg/cmd/server/start"
	"github.com/spf13/pflag"
	"io"
)

// user options for openshift-diagnostics master command
type MasterDiagnosticsOptions struct {
	DiagOptions *DiagnosticsOptions
	MustCheck   bool // set for "diagnostics master" which requires diagnosing master even if there is no config file
	// reuse the master options from "openshift start master"
	MasterStartOptions *start.MasterOptions
}

// definitions used to bind the options to actual flags on a command
type MasterDiagnosticsFlagInfos struct {
	ConfigFile FlagInfo
}

// supply output writer or pre-created DiagnosticsOptions
func NewMasterDiagnosticsOptions(out io.Writer, opts *DiagnosticsOptions) *MasterDiagnosticsOptions {
	if opts != nil {
		return &MasterDiagnosticsOptions{
			DiagOptions: opts,
		}
	} else if out != nil {
		return &MasterDiagnosticsOptions{
			DiagOptions: NewDiagnosticsOptions(out),
		}
	}
	return nil
}

// default overrideable flag specifications to be bound to options.
func NewMasterDiagnosticsFlagInfos() *MasterDiagnosticsFlagInfos {
	return &MasterDiagnosticsFlagInfos{
		ConfigFile: FlagInfo{FlagMasterConfigName, "", "", "Location of the master configuration file to run from. When running from a configuration file, all other command-line arguments are ignored."},
	}
}

func (o *MasterDiagnosticsOptions) BindFlags(cmdFlags *pflag.FlagSet, flagInfos *MasterDiagnosticsFlagInfos) {
	flagInfos.ConfigFile.BindStringFlag(cmdFlags, &o.MasterStartOptions.ConfigFile)
}
