package flags

import (
	"testing"

	"k8s.io/kubernetes/pkg/util/sets"
)

func TestExpand(t *testing.T) {
	flag := NewComponentFlag(nil, "1", "2")
	if x := flag.Expand("1"); !x.Equal(sets.NewString("1")) {
		t.Fatalf("unexpected: %#v", x)
	}
	if x := flag.Expand("2"); !x.Equal(sets.NewString("2")) {
		t.Fatalf("unexpected: %#v", x)
	}
	if x := flag.Expand("1,2"); !x.Equal(sets.NewString("1", "2")) {
		t.Fatalf("unexpected: %#v", x)
	}
	if x := flag.Expand("3"); !x.Equal(sets.NewString("3")) {
		t.Fatalf("unexpected: %#v", x)
	}
	flag = NewComponentFlag(map[string][]string{"1": {"3", "4"}}, "3", "2", "4")
	if x := flag.Expand("1"); !x.Equal(sets.NewString("3", "4")) {
		t.Fatalf("unexpected: %#v", x)
	}
	if x := flag.Expand("1,2"); !x.Equal(sets.NewString("3", "4", "2")) {
		t.Fatalf("unexpected: %#v", x)
	}
}

func TestActual(t *testing.T) {
	flag := NewComponentFlag(nil, "1", "2")
	flag.enabled = "1"
	if x, err := flag.Validate(); err != nil || !x.Equal(sets.NewString("1")) {
		t.Fatalf("unexpected %#v: %v", x, err)
	}
	flag = NewComponentFlag(nil, "1", "2")
	flag.enabled = ""
	if x, err := flag.Validate(); err != nil || !x.Equal(sets.NewString()) {
		t.Fatalf("unexpected %#v: %v", x, err)
	}
	flag = NewComponentFlag(nil, "1", "2")
	flag.enabled = "1,2,3"
	if x, err := flag.Validate(); err == nil || !x.Equal(sets.NewString()) {
		t.Fatalf("unexpected %#v: %v", x, err)
	}
	flag = NewComponentFlag(nil, "1", "2")
	flag.enabled = "3"
	if x, err := flag.Validate(); err == nil || !x.Equal(sets.NewString()) {
		t.Fatalf("unexpected %#v: %v", x, err)
	}
	flag = NewComponentFlag(map[string][]string{"3": {"1", "2"}}, "1", "2")
	flag.enabled = "3"
	if x, err := flag.Validate(); err != nil || !x.Equal(sets.NewString("1", "2")) {
		t.Fatalf("unexpected %#v: %v", x, err)
	}
	flag = NewComponentFlag(map[string][]string{"3": {"1", "2"}}, "1", "2")
	flag.enabled = "1,3"
	if x, err := flag.Validate(); err != nil || !x.Equal(sets.NewString("1", "2")) {
		t.Fatalf("unexpected %#v: %v", x, err)
	}
}
