package user

import (
	"testing"
)

func TestParseRangeList(t *testing.T) {
	tests := []struct {
		str         string
		expected    *RangeList
		errExpected bool
	}{
		{
			str: "1-5,5,2-",
			expected: &RangeList{
				validRange(NewRange(1, 5)),
				validRange(NewRange(5, 5)),
				validRange(NewRangeFrom(2)),
			},
		},
		{
			str: ",4-5,0-",
			expected: &RangeList{
				&Range{},
				validRange(NewRange(4, 5)),
				validRange(NewRangeFrom(0)),
			},
		},
		{
			str: "5",
			expected: &RangeList{
				validRange(NewRange(5, 5)),
			},
		},
		{
			str: "1-",
			expected: &RangeList{
				validRange(NewRangeFrom(1)),
			},
		},
		{
			str: "-5",
			expected: &RangeList{
				validRange(NewRangeTo(5)),
			},
		},
		{
			str:         "abc",
			errExpected: true,
		},
		{
			str:         "{1-5}",
			errExpected: true,
		},
		{
			str:         "1-5-,2-",
			errExpected: true,
		},
	}

	for _, tc := range tests {
		actual, err := ParseRangeList(tc.str)
		if err != nil {
			if !tc.errExpected {
				t.Errorf("Unexpected error for input %s: %v", tc.str, err)
			}
			continue
		}
		if tc.errExpected {
			t.Errorf("Expected error but did not get one for input %s", tc.str)
			continue
		}
		if actual.String() != tc.expected.String() {
			t.Errorf("Did not get expected range list for input %s. Expected: %v, Got: %v",
				tc.str, tc.expected, actual)
		}
	}
}

func TestRangeListContains(t *testing.T) {
	tests := []struct {
		uid      int
		r        *RangeList
		expected bool
	}{
		{
			uid: 10,
			r: &RangeList{
				validRange(NewRange(0, 9)),
				validRange(NewRange(10, 10)),
				validRange(NewRange(20, 30)),
			},
			expected: true,
		},
		{
			uid: 5,
			r: &RangeList{
				validRange(NewRange(3, 4)),
				validRange(NewRange(6, 7)),
			},
			expected: false,
		},
		{
			uid: 10,
			r: &RangeList{
				validRange(NewRangeFrom(11)),
				validRange(NewRangeFrom(20)),
			},
			expected: false,
		},
		{
			uid:      5000,
			r:        &RangeList{},
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
