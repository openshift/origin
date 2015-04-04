package options

import (
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/spf13/pflag"
	"strconv"
)

type FlagInfo kclientcmd.FlagInfo // reuse to add methods

// FlagInfos serve as a customizable intermediary between the command flags and
// the options object they feed into. This enables reuse of the flags and options
// with tweaked definitions in different contexts if necessary.

func (i FlagInfo) BindStringFlag(flags *pflag.FlagSet, target *string) {
	// assume flags with no longname are not desired
	if len(i.LongName) > 0 {
		flags.StringVarP(target, i.LongName, i.ShortName, i.Default, i.Description)
	}
}

func (i FlagInfo) BindIntFlag(flags *pflag.FlagSet, target *int) {
	// assume flags with no longname are not desired
	if len(i.LongName) > 0 {
		// try to parse Default as an int.  If it fails, assume 0
		intVal, _ := strconv.ParseInt(i.Default, 10, 0)
		flags.IntVarP(target, i.LongName, i.ShortName, int(intVal), i.Description)
	}
}

func (i FlagInfo) BindBoolFlag(flags *pflag.FlagSet, target *bool) {
	// assume flags with no longname are not desired
	if len(i.LongName) > 0 {
		// try to parse Default as a bool.  If it fails, assume false
		boolVal, _ := strconv.ParseBool(i.Default)
		flags.BoolVarP(target, i.LongName, i.ShortName, boolVal, i.Description)
	}
}

func (i FlagInfo) BindListFlag(flags *pflag.FlagSet, target *kutil.StringList) {
	// assume flags with no longname are not desired
	if len(i.LongName) > 0 {
		flags.VarP(target, i.LongName, i.ShortName, i.Description)
	}
}

const (
	FlagAllClientConfigName = "client-config"
	FlagAllMasterConfigName = "master-config"
	FlagAllNodeConfigName   = "node-config"
	FlagDiagnosticsName     = "diagnostics"
	FlagLevelName           = "diaglevel"
	FlagFormatName          = "output"
	FlagMasterConfigName    = "config"
	FlagNodeConfigName      = "config"
)
