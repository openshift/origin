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
	local        bool
	tearDownFunc func(testing.TB, bool)
}

func PrintEventsOnFailure(namespace string) func(*E) {
	return func(e *E) {
		e.registerTearDownFun(setUpTearDown(namespace))
	}
}

func NewE(t *testing.T, options ...func(*E)) *E {
	e := &E{T: t}
	// the test logger only prints text if a test fails or the -v flag is set
	// that means we don't have any visibility when running the tests from a local machine
	//
	// thus std logger will be used when the test are run from a local machine to give instant feedback
	if len(os.Getenv("OPENSHIFT_BUILD_COMMIT")) == 0 {
		e.local = true
	}

	for _, option := range options {
		option(e)
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
		e.handleTearDown(true)
		os.Exit(-1)
	}
	e.T.Errorf(format, args...)
	e.handleTearDown(e.Failed())
}

func (e *E) Error(args ...interface{}) {
	if e.local {
		e.Errorf("%v", args...)
	}
	e.Errorf("%v", args...)
}

func (e *E) Fatalf(format string, args ...interface{}) {
	panic("Use require.NoError instead of t.Fatal so that TearDown can dump debugging info on failure")
}

func (e *E) Fatal(args ...interface{}) {
	panic("Use require.NoError instead of t.Fatal so that TearDown can dump debugging info on failure")
}

func (e *E) registerTearDownFun(tearDownFunc func(testing.TB, bool)) {
	e.tearDownFunc = tearDownFunc
}

func (e *E) handleTearDown(failed bool) {
	if e.tearDownFunc != nil {
		e.tearDownFunc(e, failed)
	}
}
