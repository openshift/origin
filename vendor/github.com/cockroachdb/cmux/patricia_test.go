package cmux

import (
	"strings"
	"testing"
)

func testPTree(t *testing.T, strs ...string) {
	pt := newPatriciaTreeString(strs...)
	for _, s := range strs {
		if !pt.match(strings.NewReader(s)) {
			t.Errorf("%s is not matched by %s", s, s)
		}

		if !pt.matchPrefix(strings.NewReader(s + s)) {
			t.Errorf("%s is not matched as a prefix by %s", s+s, s)
		}

		if pt.match(strings.NewReader(s + s)) {
			t.Errorf("%s matches %s", s+s, s)
		}
	}
}

func TestPatriciaOnePrefix(t *testing.T) {
	testPTree(t, "prefix")
}

func TestPatriciaNonOverlapping(t *testing.T) {
	testPTree(t, "foo", "bar", "dummy")
}

func TestPatriciaOverlapping(t *testing.T) {
	testPTree(t, "foo", "far", "farther", "boo", "bar")
}
