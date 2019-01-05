package resources

import (
	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/resourcefilters"
)

// Reducer applies some operation to possibly combine an accumulator (acc) and a resource (x), returning
// either the combined result or nil.
type Reducer func(acc, x *mesos.Resource) *mesos.Resource

// If applies a filter func to a resource reducer; rejected resources are not processed by the receiving reducer.
func (rf Reducer) If(f func(*mesos.Resource) bool) Reducer {
	if f == nil {
		return rf
	}
	return func(acc, x *mesos.Resource) *mesos.Resource {
		if f(x) {
			return rf(acc, x)
		}
		return acc
	}
}

// IfNot applies a filter func to a resource reducer; accepted resources are not processed by the receiving reducer.
func (rf Reducer) IfNot(f func(*mesos.Resource) bool) Reducer {
	if f == nil {
		return rf
	}
	return rf.If(func(r *mesos.Resource) bool {
		return !f(r)
	})
}

// Reduce applies the given Reducer to produce a final Resource, iterating left-to-right over the given
// resources; panics if the Reducer is nil.
func Reduce(rf Reducer, rs ...mesos.Resource) (r *mesos.Resource) {
	if rf == nil {
		panic("Reduce: reducer func may not be nil")
	}
	for i := range rs {
		r = rf(r, &rs[i])
	}
	return
}

func Sum(fs ...resourcefilters.Filter) Reducer {
	return Reducer(func(acc, x *mesos.Resource) *mesos.Resource {
		p := acc
		if p == nil {
			p = x
		}
		if p == nil {
			return nil
		}
		switch p.GetType() {
		case mesos.SCALAR:
			return &mesos.Resource{Scalar: acc.GetScalar().Add(x.GetScalar())}
		case mesos.RANGES:
			return &mesos.Resource{Ranges: acc.GetRanges().Add(x.GetRanges())}
		case mesos.SET:
			return &mesos.Resource{Set: acc.GetSet().Add(x.GetSet())}
		default:
			// we can't take the sum of TEXT type
		}
		return nil
	}).If(resourcefilters.Filters(fs).Accepts)
}
