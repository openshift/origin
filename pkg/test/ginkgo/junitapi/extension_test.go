package junitapi

import (
	"testing"
	"time"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
)

func TestToExtensionTestResults(t *testing.T) {
	tests := []struct {
		name           string
		junits         []*JUnitTestCase
		expectedCount  int
		expectedResult extensiontests.Result
		expectedName   string
		expectedError  string
		expectedOutput string
	}{
		{
			name:          "nil slice",
			junits:        nil,
			expectedCount: 0,
		},
		{
			name:          "empty slice",
			junits:        []*JUnitTestCase{},
			expectedCount: 0,
		},
		{
			name: "passed test",
			junits: []*JUnitTestCase{
				{
					Name:      "test passed",
					Duration:  5.0,
					SystemOut: "test output",
				},
			},
			expectedCount:  1,
			expectedResult: extensiontests.ResultPassed,
			expectedName:   "test passed",
			expectedOutput: "test output",
		},
		{
			name: "failed test",
			junits: []*JUnitTestCase{
				{
					Name:      "test failed",
					Duration:  10.5,
					SystemOut: "test output before failure",
					FailureOutput: &FailureOutput{
						Message: "assertion failed",
						Output:  "detailed failure info",
					},
				},
			},
			expectedCount:  1,
			expectedResult: extensiontests.ResultFailed,
			expectedName:   "test failed",
			expectedError:  "detailed failure info",
			expectedOutput: "test output before failure",
		},
		{
			name: "skipped test",
			junits: []*JUnitTestCase{
				{
					Name:      "test skipped",
					Duration:  0.1,
					SystemOut: "skip reason details",
					SkipMessage: &SkipMessage{
						Message: "not applicable",
					},
				},
			},
			expectedCount:  1,
			expectedResult: extensiontests.ResultSkipped,
			expectedName:   "test skipped",
			expectedOutput: "not applicable\n\nskip reason details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := ToExtensionTestResults(tt.junits)

			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
				return
			}

			if tt.expectedCount == 0 {
				return
			}

			result := results[0]
			if result.Name != tt.expectedName {
				t.Errorf("Name = %q, want %q", result.Name, tt.expectedName)
			}
			if result.Result != tt.expectedResult {
				t.Errorf("Result = %q, want %q", result.Result, tt.expectedResult)
			}
			if tt.expectedError != "" && result.Error != tt.expectedError {
				t.Errorf("Error = %q, want %q", result.Error, tt.expectedError)
			}
			if tt.expectedOutput != "" && result.Output != tt.expectedOutput {
				t.Errorf("Output = %q, want %q", result.Output, tt.expectedOutput)
			}
		})
	}
}

func TestToExtensionTestResults_timestamps(t *testing.T) {
	junits := []*JUnitTestCase{
		{
			Name:      "test with timestamps",
			Duration:  30.5,
			StartTime: "2023-12-25T10:00:00Z",
			EndTime:   "2023-12-25T10:00:30Z",
			Lifecycle: "informing",
		},
	}

	results := ToExtensionTestResults(junits)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]

	// Check duration conversion (seconds to milliseconds)
	expectedDuration := int64(30500) // 30.5 seconds = 30500 ms
	if result.Duration != expectedDuration {
		t.Errorf("Duration = %d, want %d", result.Duration, expectedDuration)
	}

	// Check lifecycle was parsed
	if result.Lifecycle != extensiontests.LifecycleInforming {
		t.Errorf("Lifecycle = %q, want %q", result.Lifecycle, extensiontests.LifecycleInforming)
	}

	// Check start time was parsed
	if result.StartTime == nil {
		t.Error("StartTime should not be nil")
	} else {
		expectedStart := time.Date(2023, 12, 25, 10, 0, 0, 0, time.UTC)
		if !time.Time(*result.StartTime).Equal(expectedStart) {
			t.Errorf("StartTime = %v, want %v", time.Time(*result.StartTime), expectedStart)
		}
	}

	// Check end time was parsed
	if result.EndTime == nil {
		t.Error("EndTime should not be nil")
	} else {
		expectedEnd := time.Date(2023, 12, 25, 10, 0, 30, 0, time.UTC)
		if !time.Time(*result.EndTime).Equal(expectedEnd) {
			t.Errorf("EndTime = %v, want %v", time.Time(*result.EndTime), expectedEnd)
		}
	}
}
