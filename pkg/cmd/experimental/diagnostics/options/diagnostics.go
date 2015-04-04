package options

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/spf13/pflag"
	"io"
)

// all of the diagnostics commands will bind these options
type DiagnosticsOptions struct {
	Diagnostics *util.StringList // named diagnostics to run
	DiagLevel   int              // show output of this priority or higher
	DiagFormat  string           // format of output - text/json/yaml

	Output cmdutil.Output // this is used for discovery and diagnostic output
}

func NewDiagnosticsOptions(out io.Writer) *DiagnosticsOptions {
	return &DiagnosticsOptions{
		Diagnostics: &util.StringList{}, // have to instantiate in order to bind flag
		Output:      cmdutil.Output{out},
	}
}

// definitions used to bind the options to actual flags on a command
type DiagnosticsFlagInfos struct {
	Diagnostics FlagInfo
	DiagLevel   FlagInfo
	DiagFormat  FlagInfo
}

// default overrideable flag specifications to be bound to options.
func NewDiagnosticsFlagInfos() *DiagnosticsFlagInfos {
	return &DiagnosticsFlagInfos{
		Diagnostics: FlagInfo{FlagDiagnosticsName, "d", "", `comma-separated list of diagnostic names to run, e.g. "systemd.AnalyzeLogs"`},
		DiagLevel:   FlagInfo{FlagLevelName, "l", "3", "Level of diagnostic output: 0: Error, 1: Warn, 2: Notice, 3: Info, 4: Debug"},
		DiagFormat:  FlagInfo{FlagFormatName, "o", "text", "Output format: text|json|yaml"},
	}
}

func (o *DiagnosticsOptions) BindFlags(cmdFlags *pflag.FlagSet, flagInfos *DiagnosticsFlagInfos) {
	flagInfos.Diagnostics.BindListFlag(cmdFlags, o.Diagnostics)
	flagInfos.DiagLevel.BindIntFlag(cmdFlags, &o.DiagLevel)
	flagInfos.DiagFormat.BindStringFlag(cmdFlags, &o.DiagFormat)
}
