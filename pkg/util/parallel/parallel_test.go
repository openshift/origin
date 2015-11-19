package parallel

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func TestRun(t *testing.T) {
	i := int32(0)
	errs := Run(
		func() error {
			atomic.AddInt32(&i, 1)
			return nil
		},
		func() error {
			atomic.AddInt32(&i, 5)
			return nil
		},
	)
	if len(errs) != 0 || i != 6 {
		t.Error("unexpected run")
	}

	testErr := fmt.Errorf("an error")
	i = int32(0)
	errs = Run(
		func() error {
			return testErr
		},
		func() error {
			atomic.AddInt32(&i, 5)
			return nil
		},
	)
	if len(errs) != 1 && errs[0] != testErr && i != 5 {
		t.Error("unexpected run")
	}
}
