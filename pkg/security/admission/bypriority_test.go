package admission

import (
	"sort"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
)

func TestByPriority(t *testing.T) {
	sccs := []*kapi.SecurityContextConstraints{testSCC("one", 1), testSCC("two", 2), testSCC("three", 3), testSCC("negative", -1), testSCC("super", 100)}
	expected := []string{"super", "three", "two", "one", "negative"}

	sort.Sort(ByPriority(sccs))

	for i, scc := range sccs {
		if scc.Name != expected[i] {
			t.Errorf("sort by priority found %s at element %d but expected %s", scc.Name, i, expected[i])
		}
	}
}

func TestByPrioritiesScore(t *testing.T) {
	privilegedSCC := testSCC("privileged", 1)
	privilegedSCC.AllowPrivilegedContainer = true

	nonPriviledSCC := testSCC("nonprivileged", 1)

	hostDirSCC := testSCC("hostdir", 1)
	hostDirSCC.AllowHostDirVolumePlugin = true

	sccs := []*kapi.SecurityContextConstraints{nonPriviledSCC, privilegedSCC, hostDirSCC}
	// with equal priorities expect that the SCCs will be sorted with hold behavior based on their score,
	// most restrictive first
	expected := []string{"nonprivileged", "hostdir", "privileged"}

	sort.Sort(ByPriority(sccs))

	for i, scc := range sccs {
		if scc.Name != expected[i] {
			t.Errorf("sort by score found %s at element %d but expected %s", scc.Name, i, expected[i])
		}
	}
}

func TestByPrioritiesName(t *testing.T) {
	sccs := []*kapi.SecurityContextConstraints{testSCC("e", 1), testSCC("d", 1), testSCC("a", 1), testSCC("c", 1), testSCC("b", 1)}
	// expect that with equal priorities AND an equal point value that SCCs are sorted by name
	expected := []string{"a", "b", "c", "d", "e"}

	sort.Sort(ByPriority(sccs))

	for i, scc := range sccs {
		if scc.Name != expected[i] {
			t.Errorf("sort by priority found %s at element %d but expected %s", scc.Name, i, expected[i])
		}
	}
}

func TestByPrioritiesMixedSCCs(t *testing.T) {
	privilegedSCC := testSCC("privileged", 1)
	privilegedSCC.AllowPrivilegedContainer = true

	nonPriviledSCC := testSCC("nonprivileged", 1)

	sccs := []*kapi.SecurityContextConstraints{testSCC("priorityB", 5), testSCC("priorityA", 5), testSCC("super", 100), privilegedSCC, nonPriviledSCC}
	// highest priority first, equal priority and equal score sorted by name, equal priority and non-equal score sorted most restrictive to least.
	expected := []string{"super", "priorityA", "priorityB", "nonprivileged", "privileged"}

	sort.Sort(ByPriority(sccs))

	for i, scc := range sccs {
		if scc.Name != expected[i] {
			t.Errorf("sort by priority found %s at element %d but expected %s", scc.Name, i, expected[i])
		}
	}
}

func testSCC(name string, priority int) *kapi.SecurityContextConstraints {
	return &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Priority: &priority,
	}
}
