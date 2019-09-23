package metrics

import (
	"time"
)

func InMicroseconds(d time.Duration) float64 {
	return float64(d.Nanoseconds() / time.Microsecond.Nanoseconds())
}

type (
	Counter func(...string)
	Adder   func(float64, ...string)
	Watcher func(float64, ...string)
)

// Int adds the value of `x`; convenience func for adding integers.
func (a Adder) Int(x int, s ...string) {
	a(float64(x), s...)
}

// Since records an observation of time.Now().Sub(t) in microseconds
func (w Watcher) Since(t time.Time, s ...string) {
	w(InMicroseconds(time.Now().Sub(t)), s...)
}

// Harness funcs execute the given func and record metrics concerning the execution. The error
// returned from the harness is the same error returned from the execution of the func param.
type Harness func(func() error, ...string) error

// NewHarness generates and returns an execution harness that records metrics. `counts` and `errors`
// are required; `timed` and `clock` must either both be nil, or both be non-nil.
func NewHarness(counts, errors Counter, timed Watcher, clock func() time.Time) Harness {
	var harness Harness
	if timed != nil && clock != nil {
		harness = func(f func() error, labels ...string) error {
			counts(labels...)
			var (
				t   = clock()
				err = f()
			)
			timed.Since(t, labels...)
			if err != nil {
				errors(labels...)
			}
			return err
		}
	} else {
		harness = func(f func() error, labels ...string) error {
			counts(labels...)
			err := f()
			if err != nil {
				errors(labels...)
			}
			return err
		}
	}
	return harness
}
