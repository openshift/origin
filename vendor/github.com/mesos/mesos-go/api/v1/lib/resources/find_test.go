package resources_test

import (
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib"
	rez "github.com/mesos/mesos-go/api/v1/lib/resources"
	. "github.com/mesos/mesos-go/api/v1/lib/resourcetest"
)

func TestResources_Find(t *testing.T) {
	for i, tc := range []struct {
		r1, targets, wants mesos.Resources
	}{
		{nil, nil, nil},
		{
			r1: Resources(
				Resource(Name("cpus"), ValueScalar(2), Role("role1")),
				Resource(Name("mem"), ValueScalar(10), Role("role1")),
				Resource(Name("cpus"), ValueScalar(4), Role("*")),
				Resource(Name("mem"), ValueScalar(20), Role("*")),
			),
			targets: Resources(
				Resource(Name("cpus"), ValueScalar(3), Role("role1")),
				Resource(Name("mem"), ValueScalar(15), Role("role1")),
			),
			wants: Resources(
				Resource(Name("cpus"), ValueScalar(2), Role("role1")),
				Resource(Name("mem"), ValueScalar(10), Role("role1")),
				Resource(Name("cpus"), ValueScalar(1), Role("*")),
				Resource(Name("mem"), ValueScalar(5), Role("*")),
			),
		},
		{
			r1: Resources(
				Resource(Name("cpus"), ValueScalar(1), Role("role1")),
				Resource(Name("mem"), ValueScalar(5), Role("role1")),
				Resource(Name("cpus"), ValueScalar(2), Role("role2")),
				Resource(Name("mem"), ValueScalar(8), Role("role2")),
				Resource(Name("cpus"), ValueScalar(1), Role("*")),
				Resource(Name("mem"), ValueScalar(7), Role("*")),
			),
			targets: Resources(
				Resource(Name("cpus"), ValueScalar(3), Role("role1")),
				Resource(Name("mem"), ValueScalar(15), Role("role1")),
			),
			wants: Resources(
				Resource(Name("cpus"), ValueScalar(1), Role("role1")),
				Resource(Name("mem"), ValueScalar(5), Role("role1")),
				Resource(Name("cpus"), ValueScalar(1), Role("*")),
				Resource(Name("mem"), ValueScalar(7), Role("*")),
				Resource(Name("cpus"), ValueScalar(1), Role("role2")),
				Resource(Name("mem"), ValueScalar(3), Role("role2")),
			),
		},
		{
			r1: Resources(
				Resource(Name("cpus"), ValueScalar(5), Role("role1")),
				Resource(Name("mem"), ValueScalar(5), Role("role1")),
				Resource(Name("cpus"), ValueScalar(5), Role("*")),
				Resource(Name("mem"), ValueScalar(5), Role("*")),
			),
			targets: Resources(
				Resource(Name("cpus"), ValueScalar(6)),
				Resource(Name("mem"), ValueScalar(6)),
			),
			wants: Resources(
				Resource(Name("cpus"), ValueScalar(5), Role("*")),
				Resource(Name("mem"), ValueScalar(5), Role("*")),
				Resource(Name("cpus"), ValueScalar(1), Role("role1")),
				Resource(Name("mem"), ValueScalar(1), Role("role1")),
			),
		},
		{
			r1: Resources(
				Resource(Name("cpus"), ValueScalar(1), Role("role1")),
				Resource(Name("mem"), ValueScalar(1), Role("role1")),
			),
			targets: Resources(
				Resource(Name("cpus"), ValueScalar(2), Role("role1")),
				Resource(Name("mem"), ValueScalar(2), Role("role1")),
			),
			wants: nil,
		},
	} {
		r := rez.Find(tc.targets, tc.r1...)
		Expect(t, rez.Equivalent(r, tc.wants), "test case %d failed: expected %+v instead of %+v", i, tc.wants, r)
	}
}
