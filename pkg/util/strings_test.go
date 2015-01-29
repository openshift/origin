package util

import (
	"reflect"
	"testing"
)

func TestUniqueStrings(t *testing.T) {
	cases := map[string]struct {
		In  []string
		Out []string
	}{
		"empty": {
			In:  []string{},
			Out: []string{},
		},
		"single": {
			In:  []string{"A"},
			Out: []string{"A"},
		},
		"dedup": {
			In:  []string{"A", "A", "B", "A"},
			Out: []string{"A", "B"},
		},
		"sort": {
			In:  []string{"C", "A", "A", "B", "A"},
			Out: []string{"A", "B", "C"},
		},
	}

	for k, testCase := range cases {
		out := UniqueStrings(testCase.In)
		if !reflect.DeepEqual(out, testCase.Out) {
			t.Errorf("%s: Expected %#v, got %#v", k, testCase.Out, out)
		}
	}
}
