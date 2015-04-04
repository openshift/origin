package options

import (
	"github.com/openshift/origin/pkg/cmd/server/start"
	"github.com/spf13/pflag"
	"io"
)

// user options for openshift-diagnostics node command
type NodeDiagnosticsOptions struct {
	DiagOptions *DiagnosticsOptions
	MustCheck   bool // set for "diagnostics node" which requires diagnosing node even if there is no config file
	// reuse the node options from "openshift start node"
	NodeStartOptions *start.NodeOptions
}

// definitions used to bind the options to actual flags on a command
type NodeDiagnosticsFlagInfos struct {
	ConfigFile FlagInfo
}

// supply output writer or pre-created DiagnosticsOptions
func NewNodeDiagnosticsOptions(out io.Writer, opts *DiagnosticsOptions) *NodeDiagnosticsOptions {
	if opts != nil {
		return &NodeDiagnosticsOptions{
			DiagOptions: opts,
		}
	} else if out != nil {
		return &NodeDiagnosticsOptions{
			DiagOptions: NewDiagnosticsOptions(out),
		}
	}
	return nil
}

// default overrideable flag specifications to be bound to options.
func NewNodeDiagnosticsFlagInfos() *NodeDiagnosticsFlagInfos {
	return &NodeDiagnosticsFlagInfos{
		ConfigFile: FlagInfo{FlagNodeConfigName, "", "", "Location of the node configuration file to run from. When running from a configuration file, all other command-line arguments are ignored."},
	}
}

func (o *NodeDiagnosticsOptions) BindFlags(cmdFlags *pflag.FlagSet, flagInfos *NodeDiagnosticsFlagInfos) {
	flagInfos.ConfigFile.BindStringFlag(cmdFlags, &o.NodeStartOptions.ConfigFile)
}
