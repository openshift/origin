package loglevel

import (
	"flag"
	"fmt"

	"k8s.io/klog"

	operatorv1 "github.com/openshift/api/operator/v1"
)

// LogLevelToKlog transforms operator log level to a klog numeric verbosity level.
func LogLevelToKlog(logLevel operatorv1.LogLevel) int {
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

// CurrentLogLevel attempts to guess the current log level that is used by klog.
// We can use flags here as well, but this is less ugly ano more programmatically correct than flags.
func CurrentLogLevel() operatorv1.LogLevel {
	switch {
	case klog.V(8) == true:
		return operatorv1.TraceAll
	case klog.V(6) == true:
		return operatorv1.Trace
	case klog.V(4) == true:
		return operatorv1.Debug
	case klog.V(2) == true:
		return operatorv1.Normal
	default:
		return operatorv1.Normal
	}
}

// SetVerbosityValue is a nasty hack and attempt to manipulate the global flags as klog does not expose
// a way to dynamically change the loglevel in runtime.
func SetVerbosityValue(logLevel operatorv1.LogLevel) error {
	if logLevel == CurrentLogLevel() {
		return nil
	}

	var level *klog.Level

	// Convert operator loglevel to klog numeric string
	desiredLevelValue := fmt.Sprintf("%d", LogLevelToKlog(logLevel))

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
		return level.Set(desiredLevelValue)
	}

	// Third, if modifying the flag value (which is recommended by klog) fails, then fallback to modifying
	// the internal state of klog using the empty new level.
	var newLevel klog.Level
	if err := newLevel.Set(desiredLevelValue); err != nil {
		return fmt.Errorf("failed set klog.logging.verbosity %s: %v", desiredLevelValue, err)
	}

	return nil
}
