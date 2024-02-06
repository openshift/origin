package monitortestframework

import "fmt"

// NotSupportedError represents an error when a monitor test is unsupported for the given environment.
type NotSupportedError struct {
	Reason string
}

func (e *NotSupportedError) Error() string {
	return fmt.Sprintf("not supported: %s", e.Reason)
}
