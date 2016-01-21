package vnid

import (
	"testing"
)

func TestVNIDRange(t *testing.T) {
	testCases := []struct {
		base     uint
		size     uint
		success  bool
		expected string
		included int
		excluded int
		usecase  string
	}{
		{101, 100, true, "101-200", 150, 201, "valid input"},
		{201, 100, true, "201-300", 201, 301, "valid input, check min"},
		{201, 100, true, "201-300", 300, 301, "valid input, check max"},
		{10, 1, true, "10-10", 10, 11, "input with size=1"},
		{100, 0, false, "", -1, -1, "invalid size"},
		{1, 100, false, "", -1, -1, "invalid min"},
		{1, (1 << 24), false, "", -1, -1, "invalid max"},
	}

	for i := range testCases {
		tc := &testCases[i]
		r, err := NewVNIDRange(tc.base, tc.size)
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
			if tc.included >= 0 && !r.Contains(uint(tc.included)) {
				t.Errorf("expected %q to include %d", r.String(), tc.included)
			}
			if tc.excluded >= 0 && r.Contains(uint(tc.excluded)) {
				t.Errorf("expected %q to exclude %d", r.String(), tc.excluded)
			}
		}
	}
}

func TestParseVNIDRange(t *testing.T) {
	testCases := []struct {
		input    string
		success  bool
		expected string
		usecase  string
	}{
		{"101-200", true, "101-200", "valid input"},
		{"", false, "", "invalid input"},
		{"100", false, "", "invalid input, no hyphen in the range"},
		{"100:", false, "", "invalid input, missing max value"},
		{"100/200", false, "", "invalid input, unsupported format"},
		{"0-99", false, "", "violates min vnid value"},
	}

	for i := range testCases {
		tc := &testCases[i]
		r, err := ParseVNIDRange(tc.input)
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
		}
	}
}

func TestValidVNID(t *testing.T) {
	testCases := []struct {
		input   uint
		success bool
		usecase string
	}{
		{MinVNID, true, "equal to min vnid"},
		{101, true, "greater than min vnid"},
		{0, true, "global vnid"},
		{MaxVNID - 4, true, "less than max vnid"},
		{MaxVNID, true, "equal to max vnid"},
		{4, false, "less than min vnid"},
		{MaxVNID + 4, false, "greater than max vnid"},
	}

	for i := range testCases {
		tc := &testCases[i]
		err := ValidVNID(tc.input)
		if err != nil && tc.success == true {
			t.Errorf("expected success for %s, got %q", tc.usecase, err)
			continue
		} else if err == nil && tc.success == false {
			t.Errorf("expected failure for %s", tc.usecase)
			continue
		}
	}
}
