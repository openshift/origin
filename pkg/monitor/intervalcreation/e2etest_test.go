package intervalcreation

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

func TestE2ETestIntervals(t *testing.T) {
	baseTime := time.Now()
	tests := []struct {
		name            string
		inputs          monitorapi.Intervals
		expectedOutputs monitorapi.Intervals
	}{
		{
			name: "test 1",
			inputs: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: `e2e-test/"test a" jUnitSuite/openshift-tests`,
						Message: "started",
					},
					From: baseTime.Add(-50 * time.Second),
					To:   baseTime.Add(-50 * time.Second),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: `e2e-test/"test a" jUnitSuite/openshift-tests`,
						Message: "finishedStatus/Passed",
					},
					From: baseTime.Add(-10 * time.Second),
					To:   baseTime.Add(-10 * time.Second),
				},
			},
			expectedOutputs: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: `e2e-test/"test a" jUnitSuite/openshift-tests status/Passed`,
						Message: "e2e test finished As \"Passed\"",
					},
					From: baseTime.Add(-50 * time.Second),
					To:   baseTime.Add(-10 * time.Second),
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IntervalsFromEvents_E2ETests(tc.inputs, nil, baseTime.Add(-1*time.Hour), baseTime)
			assert.Equal(t, tc.expectedOutputs, result)
		})
	}
}
