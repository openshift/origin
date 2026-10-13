// Package core provides retry and polling utilities: configurable retry logic, status polling, and timeout handling.
package core

import (
	"fmt"
	"strings"
	"time"

	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// RetryOptions configures retry behavior with timeout, poll interval, max attempts, and error filtering.
type RetryOptions struct {
	Timeout      time.Duration
	PollInterval time.Duration
	MaxRetries   int
	ShouldRetry  func(error) bool
}

// RetryWithOptions retries an operation with configurable timeout, max attempts, and error filtering.
//
//	err := RetryWithOptions(func() error { return etcdOp() }, RetryOptions{Timeout: 5*time.Minute, PollInterval: 30*time.Second}, "etcd operation")
func RetryWithOptions(operation func() error, opts RetryOptions, operationName string) error {
	startTime := time.Now()
	attemptCount := 0

	for {
		attemptCount++

		// Execute the operation
		err := operation()
		if err == nil {
			e2e.Logf("Operation '%s' succeeded on attempt %d after %v", operationName, attemptCount, time.Since(startTime))
			return nil
		}

		// Check if we should retry this error
		if opts.ShouldRetry != nil && !opts.ShouldRetry(err) {
			e2e.Logf("Operation '%s' failed with non-retriable error on attempt %d: %v", operationName, attemptCount, err)
			return err
		}

		// Check timeout condition
		if opts.Timeout > 0 && time.Since(startTime) >= opts.Timeout {
			return fmt.Errorf("operation '%s' failed after %v timeout (attempts: %d, last error: %v)",
				operationName, opts.Timeout, attemptCount, err)
		}

		// Check max retries condition
		if opts.MaxRetries > 0 && attemptCount >= opts.MaxRetries {
			return fmt.Errorf("operation '%s' failed after %d attempts (elapsed: %v, last error: %v)",
				operationName, opts.MaxRetries, time.Since(startTime), err)
		}

		// Log retry
		if opts.ShouldRetry != nil {
			e2e.Logf("Operation '%s' attempt %d failed with retriable error (retrying in %v): %v",
				operationName, attemptCount, opts.PollInterval, err)
		} else {
			e2e.Logf("Operation '%s' attempt %d failed (retrying in %v): %v",
				operationName, attemptCount, opts.PollInterval, err)
		}

		// Wait before next attempt
		time.Sleep(opts.PollInterval)
	}
}

// StatusChecker checks if a condition is met, returning (true, nil) when satisfied.
type StatusChecker func() (bool, error)

// PollOptions configures polling behavior with timeout, poll interval, and max attempts.
type PollOptions struct {
	Timeout      time.Duration
	PollInterval time.Duration
	MaxAttempts  int
}

// PollUntilWithOptions repeatedly checks a condition until it's met or limits are reached.
//
//	err := PollUntilWithOptions(func() (bool, error) { return checkReady() }, PollOptions{Timeout: 10*time.Minute, PollInterval: 30*time.Second}, "node ready")
func PollUntilWithOptions(checker StatusChecker, opts PollOptions, description string) error {
	// Wrap the StatusChecker to work with RetryWithOptions
	// The operation returns nil when condition is met, error otherwise
	operation := func() error {
		met, err := checker()
		if err != nil {
			// Return a retriable error - we'll continue polling
			return fmt.Errorf("status check error: %w", err)
		}
		if met {
			// Condition met - return nil to stop polling
			return nil
		}
		// Condition not met yet - return an error to continue polling
		return fmt.Errorf("condition not yet met")
	}

	// Use RetryWithOptions with configured polling behavior
	err := RetryWithOptions(operation, RetryOptions{
		Timeout:      opts.Timeout,
		PollInterval: opts.PollInterval,
		MaxRetries:   opts.MaxAttempts,
		// No ShouldRetry - all errors are retriable for polling
	}, description)

	// Customize the error message for polling context
	if err != nil {
		// If it's our "condition not yet met" error, make it clearer
		if strings.Contains(err.Error(), "condition not yet met") {
			if opts.MaxAttempts > 0 && opts.Timeout > 0 {
				return fmt.Errorf("condition '%s' not met within %v timeout or %d attempts", description, opts.Timeout, opts.MaxAttempts)
			} else if opts.MaxAttempts > 0 {
				return fmt.Errorf("condition '%s' not met within %d attempts", description, opts.MaxAttempts)
			} else {
				return fmt.Errorf("condition '%s' not met within %v timeout", description, opts.Timeout)
			}
		}
		// Otherwise return the error as-is (likely a status check error)
		return err
	}

	return nil
}

// PollUntil is a convenience wrapper for simple timeout-based polling.
//
//	err := PollUntil(func() (bool, error) { return checkCondition() }, 5*time.Minute, 10*time.Second, "condition")
func PollUntil(checker StatusChecker, timeout, pollInterval time.Duration, description string) error {
	return PollUntilWithOptions(checker, PollOptions{
		Timeout:      timeout,
		PollInterval: pollInterval,
	}, description)
}
