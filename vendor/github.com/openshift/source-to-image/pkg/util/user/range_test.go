package user

import (
	"testing"
)

func validRange(r *Range, err error) *Range {
	if err != nil {
		panic(err)
	}
	return r
}

func TestParseRange(t *testing.T) {
	tests := []struct {
		str         string
		expected    *Range
		errExpected bool
	}{
		{
			str:      "1-5",
			expected: validRange(NewRange(1, 5)),
		},
		{
			str:      "7",
			expected: validRange(NewRange(7, 7)),
		},
		{
			str:      "1-",
			expected: validRange(NewRangeFrom(1)),
		},
		{
			str:      "-1000",
			expected: validRange(NewRangeTo(1000)),
		},
		{
			str:      "",
			expected: &Range{},
		},
		{
			str:         "--",
			errExpected: true,
		},
		{
			str:         "4-5-1",
			errExpected: true,
		},
		{
			str:         "-1-5",
			errExpected: true,
		},
		{
			str:         "abc",
			errExpected: true,
		},
	}
	for _, tc := range tests {
		actual, err := ParseRange(tc.str)
		if err != nil {
			if !tc.errExpected {
				t.Errorf("Unexpected error for input %s: %v", tc.str, err)
			}
			continue
		}
		if tc.errExpected {
			t.Errorf("Did not get expected error for input %s", tc.str)
			continue
		}
		if actual.String() != tc.expected.String() {
			t.Errorf("Did not get expected range for input %s. Expected: %v. Got: %v", tc.str, tc.expected, actual)
		}
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		uid      int
		r        *Range
		expected bool
	}{
		{
			uid:      0,
			r:        validRange(NewRange(0, 4)),
			expected: true,
		},
		{
			uid:      0,
			r:        validRange(NewRangeFrom(1)),
			expected: false,
		},
		{
			uid:      5000,
			r:        validRange(NewRangeTo(5001)),
			expected: true,
		},
		{
			uid:      5000,
			r:        validRange(NewRangeTo(4999)),
			expected: false,
		},
		{
			uid:      5000,
			r:        &Range{},
			expected: false,
		},
	}
	for _, tc := range tests {
		actual := tc.r.Contains(tc.uid)
		if actual != tc.expected {
			t.Errorf("Unexpected contains result. Input: %d, Range: %v. Expected: %v, Got: %v.", tc.uid, tc.r, tc.expected, actual)
		}
	}
}
