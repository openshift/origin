package options

import (
	"github.com/spf13/pflag"
	"io"
)

// user options for openshift-diagnostics main command
type AllDiagnosticsOptions struct {
	DiagOptions       *DiagnosticsOptions
	ClientDiagOptions *ClientDiagnosticsOptions
	MasterDiagOptions *MasterDiagnosticsOptions
	NodeDiagOptions   *NodeDiagnosticsOptions
	ClientConfigPath  string
	MasterConfigPath  string
	NodeConfigPath    string

	// there are cases where discovery has to look up flags created indirectly
	GlobalFlags *pflag.FlagSet
}

// definitions used to bind the options to actual flags on a command
type AllDiagnosticsFlagInfos struct {
	ClientConfigPath FlagInfo
	MasterConfigPath FlagInfo
	NodeConfigPath   FlagInfo
}

func NewAllDiagnosticsOptions(out io.Writer) *AllDiagnosticsOptions {
	common := NewDiagnosticsOptions(out)

	return &AllDiagnosticsOptions{
		DiagOptions:       common,
		ClientDiagOptions: NewClientDiagnosticsOptions(nil, common),
		MasterDiagOptions: NewMasterDiagnosticsOptions(nil, common),
		NodeDiagOptions:   NewNodeDiagnosticsOptions(nil, common),
	}
}

// default overrideable flag specifications to be bound to options.
func NewAllDiagnosticsFlagInfos() *AllDiagnosticsFlagInfos {
	return &AllDiagnosticsFlagInfos{
		ClientConfigPath: FlagInfo{FlagAllClientConfigName, "", "", "Path to the client config file."},
		MasterConfigPath: FlagInfo{FlagAllMasterConfigName, "", "", "Path to the master config file."},
		NodeConfigPath:   FlagInfo{FlagAllNodeConfigName, "", "", "Path to the node config file."},
	}
}

func (o *AllDiagnosticsOptions) BindFlags(cmdFlags *pflag.FlagSet, flagInfos *AllDiagnosticsFlagInfos) {
	flagInfos.ClientConfigPath.BindStringFlag(cmdFlags, &o.ClientConfigPath)
	flagInfos.MasterConfigPath.BindStringFlag(cmdFlags, &o.MasterConfigPath)
	flagInfos.NodeConfigPath.BindStringFlag(cmdFlags, &o.NodeConfigPath)
}
