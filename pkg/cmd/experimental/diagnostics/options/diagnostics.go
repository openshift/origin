package options

import (
	"github.com/spf13/pflag"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/diagnostics/log"
)

type RecommendedLoggerOptionsFlags struct {
	Level  FlagInfo
	Format FlagInfo
}

// default overrideable flag specifications to be bound to options.
func RecommendedLoggerOptionFlags() RecommendedLoggerOptionsFlags {
	return RecommendedLoggerOptionsFlags{
		Level:  FlagInfo{FlagLevelName, "l", "1", "Level of diagnostic output: 4: Error, 3: Warn, 2: Notice, 1: Info, 0: Debug"},
		Format: FlagInfo{FlagFormatName, "o", "text", "Output format: text|json|yaml"},
	}
}

func BindLoggerOptionFlags(cmdFlags *pflag.FlagSet, loggerOptions *log.LoggerOptions, flags RecommendedLoggerOptionsFlags) {
	flags.Level.BindIntFlag(cmdFlags, &loggerOptions.Level)
	flags.Format.BindStringFlag(cmdFlags, &loggerOptions.Format)
}

// default overrideable flag specifications to be bound to options.
func NewRecommendedDiagnosticFlag() FlagInfo {
	return FlagInfo{FlagDiagnosticsName, "d", "", `comma-separated list of diagnostic names to run, e.g. "systemd.AnalyzeLogs"`}
}

func BindDiagnosticFlag(cmdFlags *pflag.FlagSet, diagnostics *util.StringList, flagInfo FlagInfo) {
	flagInfo.BindListFlag(cmdFlags, diagnostics)
}
