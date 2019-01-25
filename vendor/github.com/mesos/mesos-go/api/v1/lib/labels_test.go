package mesos

import (
	"strconv"
	"testing"
)

func TestEquivalent_Labels(t *testing.T) {
	for ti, tc := range []struct {
		l1, l2 *Labels
		wants  bool
	}{
		{wants: true},
		{l1: &Labels{}, wants: true},
		{l2: &Labels{}, wants: true},
		{l1: &Labels{}, l2: &Labels{}, wants: true},
		{l1: &Labels{}, l2: &Labels{Labels: []Label{}}, wants: true},
		{l2: &Labels{Labels: []Label{}}, wants: true},
		{l1: &Labels{Labels: []Label{}}, l2: &Labels{Labels: []Label{}}, wants: true},
		{
			l1: &Labels{Labels: []Label{
				{Key: "a"},
			}},
			l2: &Labels{Labels: []Label{
				{Key: "a"},
			}},
			wants: true,
		},
		{
			l1: &Labels{Labels: []Label{
				{Key: "c"},
				{Key: "b"},
				{Key: "a"},
			}},
			l2: &Labels{Labels: []Label{
				{Key: "a"},
				{Key: "b"},
				{Key: "c"},
			}},
			wants: true,
		},
		{
			l1: &Labels{Labels: []Label{
				{Key: "a"},
			}},
			l2: &Labels{Labels: []Label{
				{Key: "a"},
				{Key: "b"},
				{Key: "c"},
			}},
		},
		{
			l1: &Labels{Labels: []Label{
				{Key: "a"},
			}},
			l2: &Labels{Labels: []Label{
				{Key: "c"},
			}},
		},
		{
			l1: &Labels{Labels: []Label{
				{Key: "a"},
			}},
			l2: &Labels{Labels: []Label{}},
		},
		{
			l1: &Labels{Labels: []Label{
				{Key: "a"},
			}},
		},
	} {
		t.Run(strconv.Itoa(ti), func(t *testing.T) {
			eq := tc.l1.Equivalent(tc.l2)
			if eq != tc.wants {
				if tc.wants {
					t.Fatal("expected equivalent labels")
				} else {
					t.Fatal("expected non-equivalent labels")
				}
			}
		})
	}
}

func TestLabels_Format(t *testing.T) {
	ps := func(s string) *string { return &s }
	for ti, tc := range []struct {
		lab   *Labels
		wants string
	}{
		{},
		{&Labels{}, ""},
		{&Labels{Labels: []Label{}}, ""},
		{&Labels{Labels: []Label{{Key: "a"}}}, "a"},
		{&Labels{Labels: []Label{{Key: "a"}, {Key: "b"}}}, "a,b"},
		{&Labels{Labels: []Label{{Key: "a"}, {Key: "b", Value: ps("1")}, {Key: "c"}}}, "a,b=1,c"},
	} {
		t.Run(strconv.Itoa(ti), func(t *testing.T) {
			actual := tc.lab.Format()
			if tc.wants != actual {
				t.Errorf("expected %q instead of %q", tc.wants, actual)
			}
		})
	}
}
