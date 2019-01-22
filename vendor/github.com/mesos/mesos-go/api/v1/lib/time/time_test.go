package time_test

import (
	"testing"
	"time"

	. "github.com/mesos/mesos-go/api/v1/lib/time"
)

func TestParseDuration(t *testing.T) {
	for i, tc := range []struct {
		input      string
		output     time.Duration
		wantsError bool
	}{
		{wantsError: true},
		{input: "1ns", output: time.Duration(1)},
		{input: "1.5ns", output: time.Duration(1)},
		{input: "1.9ns", output: time.Duration(1)},
		{input: "1us", output: 1 * time.Microsecond},
		{input: "1.5us", output: 1*time.Microsecond + 500},
		{input: "1ms", output: 1 * time.Millisecond},
		{input: "1secs", output: 1 * time.Second},
		{input: "1mins", output: 1 * time.Minute},
		{input: "1hrs", output: 1 * time.Hour},
		{input: "1days", output: 1 * time.Hour * 24},
		{input: "1weeks", output: 1 * time.Hour * 24 * 7},
		{input: "1", wantsError: true},
		{input: "1.0", wantsError: true},
		{input: "1.0.0ns", wantsError: true},
		{input: "a1ns", wantsError: true},
		{input: "ns", wantsError: true},
		{input: "1ms1ns", wantsError: true},
	} {
		d, err := ParseDuration(tc.input)
		if err != nil && tc.wantsError {
			continue
		}
		if err != nil {
			t.Errorf("test case %d failed: unexpected error %+v", i, err)
		} else if tc.wantsError {
			t.Errorf("test case %d failed: expected error for input %q", i, tc.input)
		} else if tc.output != d {
			t.Errorf("test case %d failed: expected output %v instead of %v", i, tc.output, d)
		}
	}
}
