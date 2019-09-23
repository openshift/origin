package resources_test

import (
	"reflect"
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib"
	rez "github.com/mesos/mesos-go/api/v1/lib/resources"
	. "github.com/mesos/mesos-go/api/v1/lib/resourcetest"
)

func TestTypesOf(t *testing.T) {
	rs := Resources(
		Resource(Name("cpus"), ValueScalar(2), Role("role1")),
		Resource(Name("cpus"), ValueScalar(4)),
		Resource(Name("ports"), ValueRange(Span(1, 10)), Role("role1")),
		Resource(Name("ports"), ValueRange(Span(11, 20))),
	)
	types := rez.TypesOf(rs...)
	expected := map[rez.Name]mesos.Value_Type{
		rez.NameCPUs:  mesos.SCALAR,
		rez.NamePorts: mesos.RANGES,
	}
	if !reflect.DeepEqual(types, expected) {
		t.Fatalf("expected %v instead of %v", expected, types)
	}
}

func TestNamesOf(t *testing.T) {
	rs := Resources(
		Resource(Name("cpus"), ValueScalar(2), Role("role1")),
		Resource(Name("cpus"), ValueScalar(4)),
		Resource(Name("mem"), ValueScalar(10), Role("role1")),
		Resource(Name("mem"), ValueScalar(10)),
	)
	names := rez.NamesOf(rs...)
	rez.Names(names).Sort()
	expected := []rez.Name{rez.NameCPUs, rez.NameMem}
	if !reflect.DeepEqual(names, expected) {
		t.Fatalf("expected %v instead of %v", expected, names)
	}
}

func TestFlatten(t *testing.T) {
	for i, tc := range []struct {
		r1, wants mesos.Resources
	}{
		{nil, nil},
		{
			r1: Resources(
				Resource(Name("cpus"), ValueScalar(1), Role("role1")),
				Resource(Name("cpus"), ValueScalar(2), Role("role2")),
				Resource(Name("mem"), ValueScalar(5), Role("role1")),
			),
			wants: Resources(
				Resource(Name("cpus"), ValueScalar(3)),
				Resource(Name("mem"), ValueScalar(5)),
			),
		},
		{
			r1: Resources(
				Resource(Name("cpus"), ValueScalar(3), Role("role1")),
				Resource(Name("mem"), ValueScalar(15), Role("role1")),
			),
			wants: Resources(
				Resource(Name("cpus"), ValueScalar(3), Role("*")),
				Resource(Name("mem"), ValueScalar(15), Role("*")),
			),
		},
	} {
		r := rez.Flatten(tc.r1)
		Expect(t, rez.Equivalent(r, tc.wants), "test case %d failed: expected %+v instead of %+v", i, tc.wants, r)
	}
}
