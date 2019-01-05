package resources

import (
	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/resourcefilters"
)

func Find(wants mesos.Resources, from ...mesos.Resource) (total mesos.Resources) {
	for i := range wants {
		found := find(wants[i], from...)

		// each want *must* be found
		if len(found) == 0 {
			return nil
		}

		total.Add(found...)
	}
	return total
}

func find(want mesos.Resource, from ...mesos.Resource) mesos.Resources {
	var (
		total      = mesos.Resources(from).Clone()
		remaining  = mesos.Resources{want}.ToUnreserved()
		found      mesos.Resources
		predicates = resourcefilters.Filters{}
	)
	if want.IsReserved("") {
		predicates = append(predicates, resourcefilters.ReservedByRole(want.ReservationRole()))
	}
	predicates = append(predicates, resourcefilters.Unreserved, resourcefilters.Any)
	for _, predicate := range predicates {
		filtered := resourcefilters.Select(predicate, total...)
		for i := range filtered {
			// ToUnreserved in order to ignore roles in contains()
			unreserved := mesos.Resources{filtered[i]}.ToUnreserved()
			if ContainsAll(unreserved, remaining) {
				// want has been found, return the result
				for j := range remaining {
					r := remaining[j]
					// assume that the caller isn't mixing pre- and post-reservation
					// refinement strategies: that all resources either use one format
					// or the other.
					r.Role = filtered[i].Role
					r.Reservation = filtered[i].Reservation
					r.Reservations = filtered[i].Reservations
					found.Add1(r)
				}
				return found
			} else if ContainsAll(remaining, unreserved) {
				found.Add1(filtered[i])
				total.Subtract1(filtered[i])
				remaining.Subtract(unreserved...)
				break
			}
		}
	}
	return nil
}
