package loglevel

import operatorv1 "github.com/openshift/api/operator/v1"

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
