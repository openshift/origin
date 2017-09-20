package securitycontextconstraints

import (
	"sort"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

func TestByPriority(t *testing.T) {
	sccs := []*securityapi.SecurityContextConstraints{testSCC("one", 1), testSCC("two", 2), testSCC("three", 3), testSCC("negative", -1), testSCC("super", 100)}
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
	hostDirSCC.Volumes = []securityapi.FSType{securityapi.FSTypeHostPath}

	sccs := []*securityapi.SecurityContextConstraints{nonPriviledSCC, privilegedSCC, hostDirSCC}
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
	sccs := []*securityapi.SecurityContextConstraints{testSCC("e", 1), testSCC("d", 1), testSCC("a", 1), testSCC("c", 1), testSCC("b", 1)}
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

	sccs := []*securityapi.SecurityContextConstraints{testSCC("priorityB", 5), testSCC("priorityA", 5), testSCC("super", 100), privilegedSCC, nonPriviledSCC}
	// highest priority first, equal priority and equal score sorted by name, equal priority and non-equal score sorted most restrictive to least.
	expected := []string{"super", "priorityA", "priorityB", "nonprivileged", "privileged"}

	sort.Sort(ByPriority(sccs))

	for i, scc := range sccs {
		if scc.Name != expected[i] {
			t.Errorf("sort by priority found %s at element %d but expected %s", scc.Name, i, expected[i])
		}
	}
}

func testSCC(name string, priority int) *securityapi.SecurityContextConstraints {
	newPriority := int32(priority)
	return &securityapi.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Priority: &newPriority,
	}
}
