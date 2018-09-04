package hostindex

import (
	"sort"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/origin/pkg/route/controller/routeapihelpers"
)

// Changed allows a route activation function to record which routes moved from inactive
// to active or vice versa.
type Changed interface {
	// Activated should be invoked if the route was previously inactive and is now active.
	Activated(route *routev1.Route)
	// Displaced should be invoked if the route was previously active and is now inactive.
	Displaced(route *routev1.Route)
}

// RouteActivationFunc attempts to add routes from inactive into the active set. Any routes that are not
// valid must be returned in the displaced list. All routes that are added into active should be passed to
// Changed.Activated() and any route in active that ends up in the displaced list should be passed to
// Changed.Displaced(). It is the caller's responsiblity to invoke Activated or Displaced for inactive routes.
// Routes must be provided to the function in lowest to highest order and the ordering should be preserved
// in activated.
type RouteActivationFunc func(changed Changed, active []*routev1.Route, inactive ...*routev1.Route) (activated, displaced []*routev1.Route)

// OldestFirst identifies all unique host+port combinations in active and inactive, ordered by oldest
// first. Duplicates are returned in displaced in order.  It assumes all provided route have the same
// spec.host value.
func OldestFirst(changed Changed, active []*routev1.Route, inactive ...*routev1.Route) (updated, displaced []*routev1.Route) {
	if len(inactive) == 0 {
		return active, nil
	}

	// Routes must be unique in the updated list (no duplicate spec.host / spec.path combination).
	return zipperMerge(active, inactive, changed, func(route *routev1.Route) bool { return true })
}

// SameNamespace identifies all unique host+port combinations in active and inactive that are from
// the same namespace as the oldest route (creation timestamp and then uid ordering). Duplicates and
// non matching routes are returned in displaced and are unordered. It assumes all provided routes
// have the same spec.host value.
func SameNamespace(changed Changed, active []*routev1.Route, inactive ...*routev1.Route) (updated, displaced []*routev1.Route) {
	if len(inactive) == 0 {
		return active, nil
	}
	ns := inactive[0].Namespace

	// if:
	//   * there was no previous claimant
	//   * we're newer than the oldest claimant
	//   * or we're in the same namespace as the oldest claimant
	// add the new routes to the existing active set
	if len(active) == 0 || routeapihelpers.RouteLessThan(active[0], inactive[0]) {
		if len(active) > 0 {
			ns = active[0].Namespace
		}
		updated = active
		for _, route := range inactive {
			updated, displaced = appendRoute(changed, updated, displaced, route, ns == route.Namespace, false)
		}
		sort.Slice(updated, func(i, j int) bool { return routeapihelpers.RouteLessThan(updated[i], updated[j]) })
		return updated, displaced
	}

	// We're claiming the host and in a different namespace than the previous holder so we must recalculate
	// everything. We do that with a zipper merge of the two sorted lists, appending routes as we go.
	// Routes must be unique in the updated list (no duplicate spec.host / spec.path combination).
	ns = ""
	return zipperMerge(active, inactive, changed, func(route *routev1.Route) bool {
		if len(ns) == 0 {
			ns = route.Namespace
			return true
		}
		return ns == route.Namespace
	})
}

// zipperMerge assumes both active and inactive are in order and takes the oldest route from either
// list until all items are processed. If fn returns false the item will be skipped.
func zipperMerge(active, inactive []*routev1.Route, changed Changed, fn func(*routev1.Route) bool) (updated, displaced []*routev1.Route) {
	i, j := 0, 0
	for {
		switch {
		case j >= len(active):
			for ; i < len(inactive); i++ {
				updated, displaced = appendRoute(changed, updated, displaced, inactive[i], fn(inactive[i]), false)
			}
			return updated, displaced
		case i >= len(inactive):
			for ; j < len(active); j++ {
				updated, displaced = appendRoute(changed, updated, displaced, active[j], fn(active[j]), true)
			}
			return updated, displaced
		default:
			a, b := inactive[i], active[j]
			if routeapihelpers.RouteLessThan(a, b) {
				updated, displaced = appendRoute(changed, updated, displaced, a, fn(a), false)
				i++
			} else {
				updated, displaced = appendRoute(changed, updated, displaced, b, fn(b), true)
				j++
			}
		}
	}
}

// appendRoute adds the route to the end of the appropriate list if matches is true and no route already exists in the list
// with the same path.
func appendRoute(changed Changed, updated, displaced []*routev1.Route, route *routev1.Route, matches bool, isActive bool) ([]*routev1.Route, []*routev1.Route) {
	if matches && !hasExistingMatch(updated, route) {
		if !isActive {
			changed.Activated(route)
		}
		return append(updated, route), displaced
	}
	if isActive {
		changed.Displaced(route)
	}
	return updated, append(displaced, route)
}

// hasExistingMatch returns true if a route is in exists with the same path.
func hasExistingMatch(exists []*routev1.Route, route *routev1.Route) bool {
	for _, existing := range exists {
		if existing.Spec.Path == route.Spec.Path {
			return true
		}
	}
	return false
}
