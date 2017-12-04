package policy

import (
	"testing"

	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	"k8s.io/kubernetes/pkg/apis/rbac"
)

func binding(roleName string, subjects []rbac.Subject) *rbac.ClusterRoleBinding {
	return &rbac.ClusterRoleBinding{RoleRef: rbac.RoleRef{Name: roleName}, Subjects: subjects}
}

func subjects(names ...string) []rbac.Subject {
	r := []rbac.Subject{}
	for _, name := range names {
		r = append(r, rbac.Subject{Name: name})
	}
	return r
}

func TestDiffObjectReferenceLists(t *testing.T) {
	tests := map[string]struct {
		A             []rbac.Subject
		B             []rbac.Subject
		ExpectedOnlyA []rbac.Subject
		ExpectedOnlyB []rbac.Subject
	}{
		"empty": {},

		"matching, order-independent": {
			A: subjects("foo", "bar"),
			B: subjects("bar", "foo"),
		},

		"partial match": {
			A:             subjects("foo", "bar"),
			B:             subjects("foo", "baz"),
			ExpectedOnlyA: subjects("bar"),
			ExpectedOnlyB: subjects("baz"),
		},

		"missing": {
			A:             subjects("foo"),
			B:             subjects("bar"),
			ExpectedOnlyA: subjects("foo"),
			ExpectedOnlyB: subjects("bar"),
		},

		"remove duplicates": {
			A:             subjects("foo", "foo"),
			B:             subjects("bar", "bar"),
			ExpectedOnlyA: subjects("foo"),
			ExpectedOnlyB: subjects("bar"),
		},
	}

	for k, tc := range tests {
		onlyA, onlyB := DiffSubjects(tc.A, tc.B)
		if !kapihelper.Semantic.DeepEqual(onlyA, tc.ExpectedOnlyA) {
			t.Errorf("%s: Expected %#v, got %#v", k, tc.ExpectedOnlyA, onlyA)
		}
		if !kapihelper.Semantic.DeepEqual(onlyB, tc.ExpectedOnlyB) {
			t.Errorf("%s: Expected %#v, got %#v", k, tc.ExpectedOnlyB, onlyB)
		}
	}
}

func TestComputeUpdate(t *testing.T) {
	tests := map[string]struct {
		ExpectedBinding *rbac.ClusterRoleBinding
		ActualBinding   *rbac.ClusterRoleBinding
		ExcludeSubjects []rbac.Subject
		Union           bool

		ExpectedUpdatedBinding *rbac.ClusterRoleBinding
		ExpectedUpdateNeeded   bool
	}{
		"match without union": {
			ExpectedBinding: binding("role", subjects("a")),
			ActualBinding:   binding("role", subjects("a")),
			Union:           false,

			ExpectedUpdatedBinding: nil,
			ExpectedUpdateNeeded:   false,
		},
		"match with union": {
			ExpectedBinding: binding("role", subjects("a")),
			ActualBinding:   binding("role", subjects("a")),
			Union:           true,

			ExpectedUpdatedBinding: nil,
			ExpectedUpdateNeeded:   false,
		},

		"different roleref with identical subjects": {
			ExpectedBinding: binding("role", subjects("a")),
			ActualBinding:   binding("differentRole", subjects("a")),
			Union:           true,

			ExpectedUpdatedBinding: binding("role", subjects("a")),
			ExpectedUpdateNeeded:   true,
		},

		"extra subjects without union": {
			ExpectedBinding: binding("role", subjects("a")),
			ActualBinding:   binding("role", subjects("a", "b")),
			Union:           false,

			ExpectedUpdatedBinding: binding("role", subjects("a")),
			ExpectedUpdateNeeded:   true,
		},
		"extra subjects with union": {
			ExpectedBinding: binding("role", subjects("a")),
			ActualBinding:   binding("role", subjects("a", "b")),
			Union:           true,

			ExpectedUpdatedBinding: nil,
			ExpectedUpdateNeeded:   false,
		},

		"missing subjects without union": {
			ExpectedBinding: binding("role", subjects("a", "c")),
			ActualBinding:   binding("role", subjects("a", "b")),
			Union:           false,

			ExpectedUpdatedBinding: binding("role", subjects("a", "c")),
			ExpectedUpdateNeeded:   true,
		},
		"missing subjects with union": {
			ExpectedBinding: binding("role", subjects("a", "c")),
			ActualBinding:   binding("role", subjects("a", "b")),
			Union:           true,

			ExpectedUpdatedBinding: binding("role", subjects("a", "c", "b")),
			ExpectedUpdateNeeded:   true,
		},

		"do not add should make update unnecessary": {
			ExpectedBinding: binding("role", subjects("a", "b")),
			ActualBinding:   binding("role", subjects("a")),
			ExcludeSubjects: subjects("b"),
			Union:           false,

			ExpectedUpdatedBinding: nil,
			ExpectedUpdateNeeded:   false,
		},
		"do not add should not add": {
			ExpectedBinding: binding("role", subjects("a", "b", "c")),
			ActualBinding:   binding("role", subjects("a")),
			ExcludeSubjects: subjects("c"),
			Union:           false,

			ExpectedUpdatedBinding: binding("role", subjects("a", "b")),
			ExpectedUpdateNeeded:   true,
		},
		"do not add should not remove": {
			ExpectedBinding: binding("role", subjects("a", "b")),
			ActualBinding:   binding("role", subjects("a", "b", "c")),
			ExcludeSubjects: subjects("b"),
			Union:           false,

			ExpectedUpdatedBinding: binding("role", subjects("a", "b")),
			ExpectedUpdateNeeded:   true,
		},
		"do not add complex": {
			ExpectedBinding: binding("role", subjects("matching1", "matching2", "missing1", "missing2")),
			ActualBinding:   binding("role", subjects("matching1", "matching2", "extra1", "extra2")),
			ExcludeSubjects: subjects("matching1", "missing1", "extra1"),
			Union:           false,

			ExpectedUpdatedBinding: binding("role", subjects("matching1", "matching2", "missing2")),
			ExpectedUpdateNeeded:   true,
		},
	}

	for k, tc := range tests {
		updatedBinding, updateNeeded := computeUpdatedBinding(*tc.ExpectedBinding, *tc.ActualBinding, tc.ExcludeSubjects, tc.Union)
		if updateNeeded != tc.ExpectedUpdateNeeded {
			t.Errorf("%s: Expected\n\t%v\ngot\n\t%v", k, tc.ExpectedUpdateNeeded, updateNeeded)
		}
		if !kapihelper.Semantic.DeepEqual(updatedBinding, tc.ExpectedUpdatedBinding) {
			t.Errorf("%s: Expected\n\t%v %v\ngot\n\t%v %v", k, tc.ExpectedUpdatedBinding.RoleRef, tc.ExpectedUpdatedBinding.Subjects, updatedBinding.RoleRef, updatedBinding.Subjects)
		}
	}
}
