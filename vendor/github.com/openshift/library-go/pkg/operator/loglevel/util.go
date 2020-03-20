package loglevel

import (
	"flag"
	"fmt"

	"k8s.io/klog"

	operatorv1 "github.com/openshift/api/operator/v1"
)

// LogLevelToVerbosity transforms operator log level to a klog numeric verbosity level.
func LogLevelToVerbosity(logLevel operatorv1.LogLevel) int {
	switch logLevel {
	case operatorv1.Normal:
		return 2
	case operatorv1.Debug:
		return 4
	case operatorv1.Trace:
		return 6
	case operatorv1.TraceAll:
		return 8
	default:
		return 2
	}
}

// verbosityFn is exported so it can be unit tested
var verbosityFn = klog.V

// GetLogLevel attempts to guess the current log level that is used by klog.
// The bool value returned determine whether we were able to determine the current log level or not.
// We can use flags here as well, but this is less ugly ano more programmatically correct than flags.
func GetLogLevel() (operatorv1.LogLevel, bool) {
	switch {
	case verbosityFn(8) == true:
		return operatorv1.TraceAll, false
	case verbosityFn(6) == true:
		return operatorv1.Trace, false
	case verbosityFn(4) == true:
		return operatorv1.Debug, false
	case verbosityFn(2) == true:
		return operatorv1.Normal, false
	default:
		// this is the default log level that will be set if the operator operatorSpec does not specify one (2).
		return operatorv1.Normal, true
	}
}

// SetLogLEvel is a nasty hack and attempt to manipulate the global flags as klog does not expose
// a way to dynamically change the loglevel in runtime.
func SetLogLEvel(targetLevel operatorv1.LogLevel) error {
	var level *klog.Level

	// Convert operator loglevel to klog numeric string
	verbosity := fmt.Sprintf("%d", LogLevelToVerbosity(targetLevel))

	// First, if the '-v' was specified in command line, attempt to acquire the level pointer from it.
	if f := flag.CommandLine.Lookup("v"); f != nil {
		if flagValue, ok := f.Value.(*klog.Level); ok {
			level = flagValue
		}
	}

	// Second, if the '-v' was not set but is still present in flags defined for the command, attempt to acquire it
	// by visiting all flags.
	if level == nil {
		flag.VisitAll(func(f *flag.Flag) {
			if level != nil {
				return
			}
			if levelFlag, ok := f.Value.(*klog.Level); ok {
				level = levelFlag
			}
		})
	}

	if level != nil {
		return level.Set(verbosity)
	}

	// Third, if modifying the flag value (which is recommended by klog) fails, then fallback to modifying
	// the internal state of klog using the empty new level.
	var newLevel klog.Level
	if err := newLevel.Set(verbosity); err != nil {
		return fmt.Errorf("failed set klog.logging.verbosity %s: %v", verbosity, err)
	}

	return nil
}
