package options

import (
	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/diagnostics/log"
)

// LoggerOptionFlags enable the user to specify how they want output.
type LoggerOptionFlags struct {
	Level FlagInfo
}

// RecommendedLoggerOptionFlags provides default overrideable Logger flag specifications to be bound to options.
func RecommendedLoggerOptionFlags() LoggerOptionFlags {
	return LoggerOptionFlags{
		Level: FlagInfo{FlagLevelName, "l", "1", "Level of diagnostic output: 4: Error, 3: Warn, 2: Notice, 1: Info, 0: Debug"},
	}
}

// BindLoggerOptionFlags binds flags to LoggerOptionFlags.
func BindLoggerOptionFlags(cmdFlags *pflag.FlagSet, loggerOptions *log.LoggerOptions, flags LoggerOptionFlags) {
	flags.Level.BindIntFlag(cmdFlags, &loggerOptions.Level)
}
