package resources

import (
	"github.com/mesos/mesos-go/api/v1/lib"
)

func (n Name) Sum(resources ...mesos.Resource) (*mesos.Resource, bool) {
	v := Reduce(Sum(n.Filter), resources...)
	if v != nil {
		return v, true
	}
	return nil, false
}

func CPUs(resources ...mesos.Resource) (float64, bool) {
	v, ok := NameCPUs.Sum(resources...)
	return v.GetScalar().GetValue(), ok
}

func GPUs(resources ...mesos.Resource) (uint64, bool) {
	v, ok := NameGPUs.Sum(resources...)
	return uint64(v.GetScalar().GetValue()), ok
}

func Memory(resources ...mesos.Resource) (uint64, bool) {
	v, ok := NameMem.Sum(resources...)
	return uint64(v.GetScalar().GetValue()), ok
}

func Disk(resources ...mesos.Resource) (uint64, bool) {
	v, ok := NameDisk.Sum(resources...)
	return uint64(v.GetScalar().GetValue()), ok
}

func Ports(resources ...mesos.Resource) (mesos.Ranges, bool) {
	v, ok := NamePorts.Sum(resources...)
	return mesos.Ranges(v.GetRanges().GetRange()), ok
}

func TypesOf(resources ...mesos.Resource) map[Name]mesos.Value_Type {
	m := map[Name]mesos.Value_Type{}
	for i := range resources {
		name := Name(resources[i].GetName())
		m[name] = resources[i].GetType() // TODO(jdef) check for conflicting types?
	}
	return m
}

func NamesOf(resources ...mesos.Resource) (names []Name) {
	m := map[Name]struct{}{}
	for i := range resources {
		n := Name(resources[i].GetName())
		if _, ok := m[n]; !ok {
			m[n] = struct{}{}
			names = append(names, n)
		}
	}
	return
}

func SumAndCompare(expected []mesos.Resource, resources ...mesos.Resource) bool {
	// from: https://github.com/apache/mesos/blob/master/src/common/resources.cpp
	// This is a sanity check to ensure the amount of each type of
	// resource does not change.
	type total struct {
		v  *mesos.Resource
		t  mesos.Value_Type
		ok bool
	}
	calcTotals := func(r []mesos.Resource) (m map[Name]total) {
		m = make(map[Name]total)
		for n, t := range TypesOf(expected...) {
			v, ok := n.Sum(expected...)
			m[n] = total{v, t, ok}
		}
		return
	}
	var (
		et = calcTotals(expected)
		rt = calcTotals(resources)
	)
	for n, tot := range et {
		r, ok := rt[n]
		if !ok {
			return false
		}
		if tot.t != r.t || tot.ok != r.ok {
			return false
		}
		switch r.t {
		case mesos.SCALAR:
			if v1, v2 := tot.v.GetScalar().GetValue(), r.v.GetScalar().GetValue(); v1 != v2 {
				return false
			}
		case mesos.RANGES:
			v1, v2 := mesos.Ranges(tot.v.GetRanges().GetRange()), mesos.Ranges(r.v.GetRanges().GetRange())
			if !v1.Equivalent(v2) { // TODO(jdef): assumes that v1 and v2 are in sort-order, is that guaranteed here?
				return false
			}
		case mesos.SET:
			if tot.v.GetSet().Compare(r.v.GetSet()) != 0 {
				return false
			}
		default:
			// noop; we don't know how to sum other types, so ignore...
		}
		delete(rt, n)
	}
	return len(rt) == 0
}

type (
	// FlattenOpt is a functional option for resource flattening, via Flatten. Implementations are expected to
	// type-narrow the given interface, matching against WithRole, WithReservation or both methods (see Role.Assign)
	FlattenOpt func(interface{})

	flattenConfig struct {
		role        string
		reservation *mesos.Resource_ReservationInfo
	}
)

// WithRole is for use w/ pre-reservation-refinement
func (fc *flattenConfig) WithRole(role string) { fc.role = role }

// WithReservation is for use w/ pre-reservation-refinement
func (fc *flattenConfig) WithReservation(r *mesos.Resource_ReservationInfo) { fc.reservation = r }

// Flatten is deprecated and should only be used when dealing w/ resources in pre-reservation-refinement format.
func Flatten(resources []mesos.Resource, opts ...FlattenOpt) []mesos.Resource {
	var flattened mesos.Resources
	fc := &flattenConfig{}
	for _, f := range opts {
		f(fc)
	}
	if fc.role == "" {
		fc.role = mesos.DefaultRole
	}
	// we intentionally manipulate a copy 'r' of the item in resources
	for _, r := range resources {
		r.Role = &fc.role
		r.Reservation = fc.reservation
		flattened.Add1(r)
	}
	return flattened
}
