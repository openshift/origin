package monitortestframework

import "fmt"

// NotSupportedError represents an error when a monitor test is unsupported for the given environment.
type NotSupportedError struct {
	Reason string
}

func (e *NotSupportedError) Error() string {
	return fmt.Sprintf("not supported: %s", e.Reason)
}

// FlakeError represents an error when a flake junit should be created for a monitor test.
type FlakeError struct {
	Err error
}

func (e *FlakeError) Error() string {
	return fmt.Sprintf("test flake with error: %v", e.Err)
}
