package resourcefilters

import (
	"github.com/mesos/mesos-go/api/v1/lib"
)

type (
	Interface interface {
		Accepts(*mesos.Resource) bool
	}
	Filter  func(*mesos.Resource) bool
	Filters []Filter
)

var _ = Interface(Filter(nil))

func (f Filter) Accepts(r *mesos.Resource) bool {
	if f != nil {
		return f(r)
	}
	return true
}

func Any(r *mesos.Resource) bool {
	return r != nil && !r.IsEmpty()
}

func Unreserved(r *mesos.Resource) bool {
	return r.IsUnreserved()
}

func PersistentVolumes(r *mesos.Resource) bool {
	return r.IsPersistentVolume()
}

func Revocable(r *mesos.Resource) bool {
	return r.IsRevocable()
}

func Scalar(r *mesos.Resource) bool {
	return r.GetType() == mesos.SCALAR
}

func Range(r *mesos.Resource) bool {
	return r.GetType() == mesos.RANGES
}

func Set(r *mesos.Resource) bool {
	return r.GetType() == mesos.SET
}

func (f Filter) OrElse(other Filter) Filter {
	return Filter(func(r *mesos.Resource) bool {
		return f.Accepts(r) || other.Accepts(r)
	})
}

func (f Filter) And(other Filter) Filter {
	if f == nil {
		if other == nil {
			return nil
		}
		return other
	}
	if other == nil {
		return f
	}
	return Filter(func(r *mesos.Resource) bool {
		return f.Accepts(r) && other.Accepts(r)
	})
}

func Select(rf Interface, resources ...mesos.Resource) (result mesos.Resources) {
	for i := range resources {
		if rf.Accepts(&resources[i]) {
			result.Add1(resources[i])
		}
	}
	return
}

func (rf Filters) Accepts(r *mesos.Resource) bool {
	for _, f := range rf {
		if !f.Accepts(r) {
			return false
		}
	}
	return true
}

var _ = Interface(Filters(nil))

func ReservedByRole(role string) Filter {
	return Filter(func(r *mesos.Resource) bool {
		return r.IsReserved(role)
	})
}

// New concatenates the given filters
func New(filters ...Filter) Filters { return Filters(filters) }
