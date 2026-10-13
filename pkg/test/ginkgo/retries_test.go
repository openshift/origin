package ginkgo

import (
	"testing"
	"time"
)

// createAttempts creates a slice of test attempts with the specified number of successes and failures
func createAttempts(successes, failures int) []*testCase {
	var attempts []*testCase

	// Add failures first
	for i := 0; i < failures; i++ {
		attempts = append(attempts, &testCase{
			name:   "test-failure",
			failed: true,
		})
	}

	// Add successes
	for i := 0; i < successes; i++ {
		attempts = append(attempts, &testCase{
			name:    "test-success",
			success: true,
		})
	}

	return attempts
}

func TestAggressiveRetryStrategy_EarlyTermination(t *testing.T) {
	strategy := NewAggressiveRetryStrategy(10, 3) // max 10 retries, need 3 failures to fail

	tests := []struct {
		name             string
		existingAttempts []*testCase
		attemptNumber    int
		expectedContinue bool
		description      string
	}{
		{
			name:             "should_continue_early_attempts",
			existingAttempts: createAttempts(0, 1), // 1 failure
			attemptNumber:    2,
			expectedContinue: true,
			description:      "Should continue when we have 1 failure and can still reach 3",
		},
		{
			name:             "should_stop_when_impossible_to_fail",
			existingAttempts: createAttempts(8, 1), // 1 failure, 8 successes = 9 attempts total
			attemptNumber:    10,                   // on attempt 10 (last possible)
			expectedContinue: false,
			description:      "Should stop when only 1 attempt left but need 2 more failures (impossible)",
		},
		{
			name:             "should_continue_when_still_possible",
			existingAttempts: createAttempts(6, 1), // 1 failure, 6 successes = 7 attempts total
			attemptNumber:    8,                    // on attempt 8
			expectedContinue: true,
			description:      "Should continue when 3 attempts left and need 2 more failures (possible)",
		},
		{
			name:             "should_stop_when_threshold_reached",
			existingAttempts: createAttempts(0, 3), // 3 failures
			attemptNumber:    4,
			expectedContinue: false,
			description:      "Should stop when failure threshold is already reached",
		},
		{
			name:             "should_stop_at_max_attempts",
			existingAttempts: createAttempts(9, 1), // 1 failure, 9 successes = 10 attempts total
			attemptNumber:    11,                   // on attempt 11 (exceeds max of 10)
			expectedContinue: false,
			description:      "Should stop when max attempts exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the first attempt as the original test case, all attempts as allAttempts
			originalTest := tt.existingAttempts[0]
			result := strategy.ShouldContinue(originalTest, tt.existingAttempts, tt.attemptNumber)
			if result != tt.expectedContinue {
				t.Errorf("%s: expected %v, got %v", tt.description, tt.expectedContinue, result)
			}
		})
	}
}

func TestAggressiveRetryStrategy_EarlyTerminationEdgeCases(t *testing.T) {
	strategy := NewAggressiveRetryStrategy(5, 2) // max 5 retries, need 2 failures to fail

	tests := []struct {
		name             string
		existingAttempts []*testCase
		attemptNumber    int
		expectedContinue bool
		description      string
	}{
		{
			name:             "exactly_at_boundary",
			existingAttempts: createAttempts(3, 1), // 1 failure, 3 successes = 4 attempts total
			attemptNumber:    5,                    // on attempt 5 (last possible)
			expectedContinue: true,
			description:      "Should continue when exactly at boundary - 1 failure + 1 remaining = 2 possible",
		},
		{
			name:             "just_past_boundary",
			existingAttempts: createAttempts(4, 1), // 1 failure, 4 successes = 5 attempts total
			attemptNumber:    6,                    // on attempt 6 (exceeds max)
			expectedContinue: false,
			description:      "Should stop when past max attempts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the first attempt as the original test case, all attempts as allAttempts
			originalTest := tt.existingAttempts[0]
			result := strategy.ShouldContinue(originalTest, tt.existingAttempts, tt.attemptNumber)
			if result != tt.expectedContinue {
				t.Errorf("%s: expected %v, got %v", tt.description, tt.expectedContinue, result)
			}
		})
	}
}

func TestAggressiveRetryStrategy_DurationLimit(t *testing.T) {
	strategy := NewAggressiveRetryStrategy(10, 3)

	// Create attempts where the original test exceeds duration limit
	attempts := []*testCase{
		{
			name:     "test-long-duration",
			duration: 5 * time.Minute, // exceeds 4 minute limit
			failed:   true,
		},
	}

	result := strategy.ShouldContinue(attempts[0], attempts, 2)
	if result != false {
		t.Errorf("Expected false for test exceeding duration limit, got %v", result)
	}
}
