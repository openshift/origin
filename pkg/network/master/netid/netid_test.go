package netid

import (
	"testing"
)

func TestNetIDRange(t *testing.T) {
	testCases := []struct {
		min      uint32
		max      uint32
		success  bool
		expected string
		included int
		excluded int
		usecase  string
	}{
		{101, 200, true, "101-200", 150, 201, "valid input"},
		{201, 300, true, "201-300", 201, 301, "valid input, check min"},
		{201, 300, true, "201-300", 300, 301, "valid input, check max"},
		{10, 10, true, "10-10", 10, 11, "input with size=1"},
		{100, 99, false, "", -1, -1, "invalid size"},
		{1, 100, false, "", -1, -1, "invalid min"},
		{1, (1 << 25), false, "", -1, -1, "invalid max"},
	}

	for i := range testCases {
		tc := &testCases[i]
		r, err := NewNetIDRange(tc.min, tc.max)
		if err != nil && tc.success == true {
			t.Errorf("expected success for %s, got %q", tc.usecase, err)
			continue
		} else if err == nil && tc.success == false {
			t.Errorf("expected failure for %s", tc.usecase)
			continue
		} else if tc.success {
			if r.String() != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, r.String())
			}
			ok, _ := r.Contains(uint32(tc.included))
			if tc.included >= 0 && !ok {
				t.Errorf("expected %q to include %d", r.String(), tc.included)
			}
			ok, _ = r.Contains(uint32(tc.excluded))
			if tc.excluded >= 0 && ok {
				t.Errorf("expected %q to exclude %d", r.String(), tc.excluded)
			}
		}
	}
}
