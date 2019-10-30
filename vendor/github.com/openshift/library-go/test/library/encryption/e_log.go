package encryption

import (
	"fmt"
	"os"
	"testing"
)

// E is like testing.T except it overloads some methods to print to stdout
// when the encryption tests are run from a local machine
type E struct {
	*testing.T
	local bool
}

func NewE(t *testing.T) *E {
	e := &E{T: t}
	// the test logger only prints text if a test fails or the -v flag is set
	// that means we don't have any visibility when running the tests from a local machine
	//
	// thus std logger will be used when the test are run from a local machine to give instant feedback
	if len(os.Getenv("PROW_JOB_ID")) == 0 {
		e.local = true
	}

	return e
}

func (e *E) Log(args ...interface{}) {
	if e.local {
		fmt.Println(args...)
		return
	}
	e.T.Log(args...)
}

func (e *E) Logf(format string, args ...interface{}) {
	if e.local {
		fmt.Printf(fmt.Sprintf("%s\n", format), args...)
		return
	}
	e.T.Logf(format, args...)
}

func (e *E) Errorf(format string, args ...interface{}) {
	if e.local {
		e.Logf(fmt.Sprintf("ERROR: %s", format), args...)
		os.Exit(-1)
	}
	e.T.Errorf(format, args...)
}

func (e *E) Error(args ...interface{}) {
	if e.local {
		e.Errorf("%v", args...)
	}
	e.T.Error(args...)
}
