package mcs

import (
	"reflect"
	"strings"
	"testing"
)

type rangeTest struct {
	label string
	in    bool
}

func TestParseRange(t *testing.T) {
	testCases := map[string]struct {
		in    string
		errFn func(error) bool
		r     Range
		total uint64
		tests []rangeTest
	}{
		"identity range": {
			in: "test,s0/1",
			r: Range{
				prefix: "test,s0",
				n:      1024,
				k:      1,
			},
			total: 1024,
		},
		"simple range": {
			in: "s0:/2",
			r: Range{
				prefix: "s0:",
				n:      1024,
				k:      2,
			},
			total: 523776,
			tests: []rangeTest{
				{label: "c100,c3", in: false},
				{label: "s0:c100,c3", in: true},
				{label: "s0:c100,c3,c0", in: false},
				{label: "s0:c3", in: false},
				{label: "s0:c1024,c0", in: false},
			},
		},
		"limited range with full prefix": {
			in: "systemd_u:systemd_t:cupsd_t:s0:/2,10",
			r: Range{
				prefix: "systemd_u:systemd_t:cupsd_t:s0:",
				n:      10,
				k:      2,
			},
			total: 45,
			tests: []rangeTest{
				{label: "systemd_u:systemd_t:cupsd_t:s0:c100,c3", in: false},
				{label: "systemd_u:systemd_t:cupsd_t:s0:c9,c8", in: true},
			},
		},
		"NaN": {
			in:    "/a",
			errFn: func(err error) bool { return strings.Contains(err.Error(), "range not in the format") },
		},
		"zero k": {
			in:    "/0",
			errFn: func(err error) bool { return strings.Contains(err.Error(), "label length must be a positive integer") },
		},
	}

	for s, testCase := range testCases {
		r, err := ParseRange(testCase.in)
		if testCase.errFn != nil && !testCase.errFn(err) {
			t.Errorf("%s: unexpected error: %v", s, err)
			continue
		}
		if err != nil {
			continue
		}
		if r.String() != testCase.in {
			t.Errorf("%s: range.String() did not match input: %s", r.String())
		}
		if *r != testCase.r {
			t.Errorf("%s: unexpected range: %#v", s, r)
		}
		if r.Size() != testCase.total {
			t.Errorf("%s: unexpected total: %d", s, r.Size())
		}
		for _, test := range testCase.tests {
			l, err := ParseLabel(test.label)
			if err != nil {
				t.Fatal(err)
			}
			if r.Contains(l) != test.in {
				t.Errorf("%s: range contains(%s) != %t", s, l, !test.in)
			}
		}
	}
}

func TestLabel(t *testing.T) {
	if _, err := ParseLabel("s0:c9,c9"); err == nil {
		t.Errorf("unexpected non-error")
	}
	if _, err := ParseLabel("s0:ca,cb"); err == nil {
		t.Errorf("unexpected non-error")
	}

	testCases := map[string]struct {
		in     string
		expect Categories
		offset uint64
		out    string
	}{
		"identity range": {
			in:     "c0,c1",
			expect: Categories{1, 0},
			offset: 0,
			out:    "c1,c0",
		},
		"order doesn't matter": {
			in:     "c1,c0",
			expect: Categories{1, 0},
			offset: 0,
			out:    "c1,c0",
		},
		"second": {
			in:     "c2,c0",
			expect: Categories{2, 0},
			offset: 1,
			out:    "c2,c0",
		},
		"single": {
			in:     "c3",
			expect: Categories{3},
			offset: 3,
			out:    "c3",
		},
		"third": {
			in:     "c3,c0",
			expect: Categories{3, 0},
			offset: 3,
			out:    "c3,c0",
		},
		"three labels": {
			in:     "c3,c1,c0",
			expect: Categories{3, 1, 0},
			offset: 1,
			out:    "c3,c1,c0",
		},
		"three labels, second": {
			in:     "s0:c10,c0,c2",
			expect: Categories{10, 2, 0},
			offset: 121,
			out:    "s0:c10,c2,c0",
		},
		"very large": {
			in:     "c1021,c1020",
			expect: Categories{1021, 1020},
			offset: 521730,
			out:    "c1021,c1020",
		},
	}

	for s, testCase := range testCases {
		labels, err := ParseLabel(testCase.in)
		if err != nil {
			t.Errorf("%s: failed to parse labels: %v", err)
			continue
		}
		if !reflect.DeepEqual(labels.Categories, testCase.expect) {
			t.Errorf("%s: unexpected categories: %v %v", s, labels.Categories, testCase.expect)
			continue
		}
		if testCase.out != labels.String() {
			t.Errorf("%s: unexpected string: %s", s, labels.String())
			continue
		}
		v := labels.Offset()
		if v != testCase.offset {
			t.Errorf("%s: unexpected offset: %d", s, v)
			continue
		}
		categories := categoriesForOffset(testCase.offset, maxCategories, uint(len(testCase.expect)))
		if !reflect.DeepEqual(categories, labels.Categories) {
			t.Errorf("%s: could not roundtrip categories %v: %v", s, labels.Categories, categories)
		}
	}
}
