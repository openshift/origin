package policy

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func binding(roleRef kapi.ObjectReference, subjects []kapi.ObjectReference) *authorizationapi.ClusterRoleBinding {
	return &authorizationapi.ClusterRoleBinding{RoleRef: roleRef, Subjects: subjects}
}

func ref(name string) kapi.ObjectReference {
	return kapi.ObjectReference{Name: name}
}

func refs(names ...string) []kapi.ObjectReference {
	r := []kapi.ObjectReference{}
	for _, name := range names {
		r = append(r, ref(name))
	}
	return r
}

func TestDiff(t *testing.T) {
	tests := map[string]struct {
		A             []kapi.ObjectReference
		B             []kapi.ObjectReference
		ExpectedOnlyA []kapi.ObjectReference
		ExpectedOnlyB []kapi.ObjectReference
	}{
		"empty": {},

		"matching, order-independent": {
			A: refs("foo", "bar"),
			B: refs("bar", "foo"),
		},

		"partial match": {
			A:             refs("foo", "bar"),
			B:             refs("foo", "baz"),
			ExpectedOnlyA: refs("bar"),
			ExpectedOnlyB: refs("baz"),
		},

		"missing": {
			A:             refs("foo"),
			B:             refs("bar"),
			ExpectedOnlyA: refs("foo"),
			ExpectedOnlyB: refs("bar"),
		},

		"remove duplicates": {
			A:             refs("foo", "foo"),
			B:             refs("bar", "bar"),
			ExpectedOnlyA: refs("foo"),
			ExpectedOnlyB: refs("bar"),
		},
	}

	for k, tc := range tests {
		onlyA, onlyB := diff(tc.A, tc.B)
		if !kapi.Semantic.DeepEqual(onlyA, tc.ExpectedOnlyA) {
			t.Errorf("%s: Expected %#v, got %#v", k, tc.ExpectedOnlyA, onlyA)
		}
		if !kapi.Semantic.DeepEqual(onlyB, tc.ExpectedOnlyB) {
			t.Errorf("%s: Expected %#v, got %#v", k, tc.ExpectedOnlyB, onlyB)
		}
	}
}

func TestComputeUpdate(t *testing.T) {
	tests := map[string]struct {
		ExpectedBinding *authorizationapi.ClusterRoleBinding
		ActualBinding   *authorizationapi.ClusterRoleBinding
		ExcludeSubjects []kapi.ObjectReference
		Union           bool

		ExpectedUpdatedBinding *authorizationapi.ClusterRoleBinding
		ExpectedUpdateNeeded   bool
	}{
		"match without union": {
			ExpectedBinding: binding(ref("role"), refs("a")),
			ActualBinding:   binding(ref("role"), refs("a")),
			Union:           false,

			ExpectedUpdatedBinding: nil,
			ExpectedUpdateNeeded:   false,
		},
		"match with union": {
			ExpectedBinding: binding(ref("role"), refs("a")),
			ActualBinding:   binding(ref("role"), refs("a")),
			Union:           true,

			ExpectedUpdatedBinding: nil,
			ExpectedUpdateNeeded:   false,
		},

		"different roleref with identical subjects": {
			ExpectedBinding: binding(ref("role"), refs("a")),
			ActualBinding:   binding(ref("differentRole"), refs("a")),
			Union:           true,

			ExpectedUpdatedBinding: binding(ref("role"), refs("a")),
			ExpectedUpdateNeeded:   true,
		},

		"extra subjects without union": {
			ExpectedBinding: binding(ref("role"), refs("a")),
			ActualBinding:   binding(ref("role"), refs("a", "b")),
			Union:           false,

			ExpectedUpdatedBinding: binding(ref("role"), refs("a")),
			ExpectedUpdateNeeded:   true,
		},
		"extra subjects with union": {
			ExpectedBinding: binding(ref("role"), refs("a")),
			ActualBinding:   binding(ref("role"), refs("a", "b")),
			Union:           true,

			ExpectedUpdatedBinding: nil,
			ExpectedUpdateNeeded:   false,
		},

		"missing subjects without union": {
			ExpectedBinding: binding(ref("role"), refs("a", "c")),
			ActualBinding:   binding(ref("role"), refs("a", "b")),
			Union:           false,

			ExpectedUpdatedBinding: binding(ref("role"), refs("a", "c")),
			ExpectedUpdateNeeded:   true,
		},
		"missing subjects with union": {
			ExpectedBinding: binding(ref("role"), refs("a", "c")),
			ActualBinding:   binding(ref("role"), refs("a", "b")),
			Union:           true,

			ExpectedUpdatedBinding: binding(ref("role"), refs("a", "c", "b")),
			ExpectedUpdateNeeded:   true,
		},

		"do not add should make update unnecessary": {
			ExpectedBinding: binding(ref("role"), refs("a", "b")),
			ActualBinding:   binding(ref("role"), refs("a")),
			ExcludeSubjects: refs("b"),
			Union:           false,

			ExpectedUpdatedBinding: nil,
			ExpectedUpdateNeeded:   false,
		},
		"do not add should not add": {
			ExpectedBinding: binding(ref("role"), refs("a", "b", "c")),
			ActualBinding:   binding(ref("role"), refs("a")),
			ExcludeSubjects: refs("c"),
			Union:           false,

			ExpectedUpdatedBinding: binding(ref("role"), refs("a", "b")),
			ExpectedUpdateNeeded:   true,
		},
		"do not add should not remove": {
			ExpectedBinding: binding(ref("role"), refs("a", "b")),
			ActualBinding:   binding(ref("role"), refs("a", "b", "c")),
			ExcludeSubjects: refs("b"),
			Union:           false,

			ExpectedUpdatedBinding: binding(ref("role"), refs("a", "b")),
			ExpectedUpdateNeeded:   true,
		},
		"do not add complex": {
			ExpectedBinding: binding(ref("role"), refs("matching1", "matching2", "missing1", "missing2")),
			ActualBinding:   binding(ref("role"), refs("matching1", "matching2", "extra1", "extra2")),
			ExcludeSubjects: refs("matching1", "missing1", "extra1"),
			Union:           false,

			ExpectedUpdatedBinding: binding(ref("role"), refs("matching1", "matching2", "missing2")),
			ExpectedUpdateNeeded:   true,
		},
	}

	for k, tc := range tests {
		updatedBinding, updateNeeded := computeUpdatedBinding(*tc.ExpectedBinding, *tc.ActualBinding, tc.ExcludeSubjects, tc.Union)
		if updateNeeded != tc.ExpectedUpdateNeeded {
			t.Errorf("%s: Expected\n\t%v\ngot\n\t%v", k, tc.ExpectedUpdateNeeded, updateNeeded)
		}
		if !kapi.Semantic.DeepEqual(updatedBinding, tc.ExpectedUpdatedBinding) {
			t.Errorf("%s: Expected\n\t%v %v\ngot\n\t%v %v", k, tc.ExpectedUpdatedBinding.RoleRef, tc.ExpectedUpdatedBinding.Subjects, updatedBinding.RoleRef, updatedBinding.Subjects)
		}
	}
}
