package offers

import (
	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/resources"
)

type (
	// Filter accepts or rejects a mesos Offer
	Filter interface {
		Accept(*mesos.Offer) bool
	}

	// FilterFunc returns true if the given Offer passes the filter
	FilterFunc func(*mesos.Offer) bool
)

// Accept implements Filter for FilterFunc
func (f FilterFunc) Accept(o *mesos.Offer) bool {
	if f == nil {
		return true
	}
	return f(o)
}

func not(f Filter) Filter {
	return FilterFunc(func(offer *mesos.Offer) bool { return !f.Accept(offer) })
}

// ByHostname returns a Filter that accepts offers with a matching Hostname
func ByHostname(hostname string) Filter {
	if hostname == "" {
		return FilterFunc(nil)
	}
	return FilterFunc(func(o *mesos.Offer) bool {
		return o.Hostname == hostname
	})
}

// ByAttributes returns a Filter that accepts offers with an attribute set accepted by
// the provided Attribute filter func.
func ByAttributes(f func(attr []mesos.Attribute) bool) Filter {
	if f == nil {
		return FilterFunc(nil)
	}
	return FilterFunc(func(o *mesos.Offer) bool {
		return f(o.Attributes)
	})
}

func ByExecutors(f func(exec []mesos.ExecutorID) bool) Filter {
	if f == nil {
		return FilterFunc(nil)
	}
	return FilterFunc(func(o *mesos.Offer) bool {
		return f(o.ExecutorIDs)
	})
}

func ByUnavailability(f func(u *mesos.Unavailability) bool) Filter {
	if f == nil {
		return FilterFunc(nil)
	}
	return FilterFunc(func(o *mesos.Offer) bool {
		return f(o.Unavailability)
	})
}

// ContainsResources returns a filter that returns true if the Resources of an Offer
// contain the wanted Resources.
func ContainsResources(wanted mesos.Resources) Filter {
	return FilterFunc(func(o *mesos.Offer) bool {
		return resources.ContainsAll(resources.Flatten(mesos.Resources(o.Resources)), wanted)
	})
}
