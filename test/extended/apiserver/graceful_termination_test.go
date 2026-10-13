package apiserver

import (
	"fmt"
	"regexp"
	"testing"
	"time"
)

func TestParseKlogTimestamp(t *testing.T) {
	klogTimestampRegexp := regexp.MustCompile(`^[IWEF](\d{4}) (\d{2}:\d{2}:\d{2}\.\d+)`)
	currentYear := time.Now().Year()

	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "warning log line with graceful termination message",
			line:     `W0120 22:20:50.473381       1 main.go:136] Previous pod kube-apiserver-ip-10-0-48-191.us-west-2.compute.internal started at 2026-01-20 22:01:27.176604246 +0000 UTC did not terminate gracefully`,
			expected: fmt.Sprintf("%d-01-20T22:20:50Z", currentYear),
		},
		{
			name:     "info level log line",
			line:     `I0315 10:05:30.123456       1 main.go:100] some info message`,
			expected: fmt.Sprintf("%d-03-15T10:05:30Z", currentYear),
		},
		{
			name:     "error level log line",
			line:     `E1231 23:59:59.999999       1 main.go:200] error message`,
			expected: fmt.Sprintf("%d-12-31T23:59:59Z", currentYear),
		},
		{
			name:     "no klog prefix",
			line:     `some random log line without a timestamp`,
			expected: "unknown",
		},
		{
			name:     "empty line",
			line:     ``,
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseKlogTimestamp(tt.line, klogTimestampRegexp)
			if result != tt.expected {
				t.Errorf("parseKlogTimestamp() = %q, want %q", result, tt.expected)
			}
		})
	}
}
