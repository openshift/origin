package options

import (
	"github.com/spf13/pflag"

	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/diagnostics/log"
)

// LoggerOptionFlags enable the user to specify how they want output.
type LoggerOptionFlags struct {
	Level  FlagInfo
	Format FlagInfo
}

// RecommendedLoggerOptionFlags provides default overrideable Logger flag specifications to be bound to options.
func RecommendedLoggerOptionFlags() LoggerOptionFlags {
	return LoggerOptionFlags{
		Level:  FlagInfo{FlagLevelName, "l", "1", "Level of diagnostic output: 4: Error, 3: Warn, 2: Notice, 1: Info, 0: Debug"},
		Format: FlagInfo{FlagFormatName, "o", "text", "Output format: text|json|yaml"},
	}
}

// BindLoggerOptionFlags binds flags to LoggerOptionFlags.
func BindLoggerOptionFlags(cmdFlags *pflag.FlagSet, loggerOptions *log.LoggerOptions, flags LoggerOptionFlags) {
	flags.Level.BindIntFlag(cmdFlags, &loggerOptions.Level)
	flags.Format.BindStringFlag(cmdFlags, &loggerOptions.Format)
}

// NewRecommendedDiagnosticFlag provides default overrideable Diagnostic flag specifications to be bound to options.
func NewRecommendedDiagnosticFlag() FlagInfo {
	return FlagInfo{FlagDiagnosticsName, "d", "", `comma-separated list of diagnostic names to run, e.g. "AnalyzeLogs"`}
}

// BindLoggerOptionFlags binds a flag on a diagnostics command per the flagInfo.
func BindDiagnosticFlag(cmdFlags *pflag.FlagSet, diagnostics *util.StringList, flagInfo FlagInfo) {
	flagInfo.BindListFlag(cmdFlags, diagnostics)
}
