package errors

import "fmt"

// NewSuiteOutOfBoundsError returns a new SuiteOutOfBounds error for the given suite name
func NewSuiteOutOfBoundsError(name string) error {
	return &suiteOutOfBoundsError{
		suiteName: name,
	}
}

// suiteOutOfBoundsError describes the failure to place a test suite into a test suite tree because the suite
// in question is not a child of any suite in the tree
type suiteOutOfBoundsError struct {
	suiteName string
}

func (e *suiteOutOfBoundsError) Error() string {
	return fmt.Sprintf("the test suite %q could not be placed under any existing roots in the tree", e.suiteName)
}

// IsSuiteOutOfBoundsError determines if the given error was raised because a suite could not be placed
// in the test suite tree
func IsSuiteOutOfBoundsError(err error) bool {
	if err == nil {
		return false
	}

	_, ok := err.(*suiteOutOfBoundsError)
	return ok
}
