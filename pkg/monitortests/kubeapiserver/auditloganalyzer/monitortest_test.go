package auditloganalyzer

import (
	"testing"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/require"
)

func TestEvaluateAPILoadBalancerShutdownSkipped(t *testing.T) {
	tests := []struct {
		name   string
		reason string
	}{
		{name: "ROSA", reason: "not applicable to ROSA"},
		{name: "unknown cluster type", reason: "cluster type could not be determined"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analyzer := &auditLogAnalyzer{
				apiLoadBalancerSkipReason:     test.reason,
				requestsDuringShutdownChecker: CheckForRequestsDuringShutdown(),
			}

			results := analyzer.evaluateAPILoadBalancerShutdown(monitorapi.Intervals{})
			require.Len(t, results, 1)
			require.NotNil(t, results[0].SkipMessage)
			require.Equal(t, test.reason, results[0].SkipMessage.Message)
			require.Nil(t, results[0].FailureOutput)
		})
	}
}
