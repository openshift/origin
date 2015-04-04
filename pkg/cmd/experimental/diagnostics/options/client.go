package options

import (
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/spf13/pflag"
	"io"
)

// user options for openshift-diagnostics client command
type ClientDiagnosticsOptions struct {
	DiagOptions *DiagnosticsOptions
	Factory     *osclientcmd.Factory
	MustCheck   bool // set for "diagnostics client" which requires diagnosing client even there is if no config file
	// Turns out we don't need to add any flags... YET
}

// definitions used to bind the options to actual flags on a command
type ClientDiagnosticsFlagInfos struct {
	// don't need yet...
	//Something   FlagInfo
}

// supply output writer or pre-created DiagnosticsOptions
func NewClientDiagnosticsOptions(out io.Writer, opts *DiagnosticsOptions) *ClientDiagnosticsOptions {
	if opts != nil {
		return &ClientDiagnosticsOptions{
			DiagOptions: opts,
		}
	} else if out != nil {
		return &ClientDiagnosticsOptions{
			DiagOptions: NewDiagnosticsOptions(out),
		}
	}
	return nil
}

// default overrideable flag specifications to be bound to options.
func NewClientDiagnosticsFlagInfos() *ClientDiagnosticsFlagInfos {
	return &ClientDiagnosticsFlagInfos{
	//NodeConfigPath:   FlagInfo{"node-config", "", "", "Path to the node config file."},
	}
}

func (o *ClientDiagnosticsOptions) BindFlags(cmdFlags *pflag.FlagSet, flagInfos *ClientDiagnosticsFlagInfos) {
	//flagInfos.Something.BindStringFlag(cmdFlags, &o.Something)
}
