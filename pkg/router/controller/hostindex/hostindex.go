package hostindex

import (
	"sort"

	"k8s.io/apimachinery/pkg/types"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

// Interface allows access to routes in the index and makes it easy
// to know when changes to routes alter the index.
type Interface interface {
	// Add attempts to add the route to the index, returning a set of
	// changes if the index. Constraints on the index may result in
	// the route being in the Displaced list. The provided route may
	// be in either the Activated or Displaced lists or neither.
	// newRoute is true if a route with the given namespace and name
	// was not in the index prior to this call.
	Add(route *routeapi.Route) (changes Changes, newRoute bool)
	// Remove attempts to remove the route from the index, returning
	// any changes that occurred due to that operation. The provided
	// route will never be in the Activated or Displaced lists on the
	// Changes object.
	Remove(route *routeapi.Route) Changes
	// RoutesForHost returns all currently active hosts for the provided
	// route.
	RoutesForHost(host string) ([]*routeapi.Route, bool)
	// Filter iterates over all routes in the index, keeping only those
	// for which fn returs true.
	Filter(fn func(*routeapi.Route) (keep bool)) Changes
	// HostLen returns the number of hosts in the index.
	HostLen() int
}

// Changes lists all routes either activated or displaced by the
// operation.
type Changes interface {
	GetActivated() []*routeapi.Route
	GetDisplaced() []*routeapi.Route
}

type routeKey struct {
	namespace string
	name      string
}

func sameRouteForKey(a *routeapi.Route, key routeKey) bool {
	return a.Name == key.name && a.Namespace == key.namespace
}

type hostIndex struct {
	activateFn RouteActivationFunc

	hostToRoute map[string]*hostRules
	routeToHost map[routeKey]string
}

// New returns a new host index that uses the provided route activation function to determine
// which routes for a given host should be active.
func New(fn RouteActivationFunc) Interface {
	return &hostIndex{
		activateFn:  fn,
		hostToRoute: make(map[string]*hostRules),
		routeToHost: make(map[routeKey]string),
	}
}

func sameRoute(a, b *routeapi.Route) bool {
	return a.Name == b.Name && a.Namespace == b.Namespace
}

func (hi *hostIndex) Add(route *routeapi.Route) (Changes, bool) {
	changes := &routeChanges{}
	added := hi.add(route, changes)
	return changes, added
}

func (hi *hostIndex) add(route *routeapi.Route, changes *routeChanges) bool {
	host := route.Spec.Host
	key := routeKey{namespace: route.Namespace, name: route.Name}
	newRoute := true

	// if the host value changed, remove the old entry
	oldHost, ok := hi.routeToHost[key]
	if ok && oldHost != host {
		if existing, _, _, ok := hi.findRoute(oldHost, key); ok {
			hi.remove(existing, true, changes)
			newRoute = false
		}
	}
	hi.routeToHost[key] = host

	existing, rules, active, ok := hi.findRoute(host, key)
	if ok {
		newRoute = false
		switch {
		case existing.UID != route.UID:
			// means we missed a delete, so creation timestamp can change
			hi.remove(existing, false, changes)
			// uid changed, which means this is
		case existing.Spec.Path != route.Spec.Path:
			// path changed, must check to see if we displace / are displaced by another route
		default:
			// if no changes have been made, we don't need to note a change
			if existing.ResourceVersion == route.ResourceVersion {
				return false
			}
			// no other significant changes, we can update the cache and then exit
			rules.replace(existing, route)
			// a route that is active should be notified
			if active {
				changes.Activated(route)
			}
			return false
		}
	}
	if rules == nil {
		rules = &hostRules{}
		hi.hostToRoute[host] = rules
	}

	rules.add(route, hi.activateFn, changes)
	return newRoute
}

func (hi *hostIndex) findRoute(host string, key routeKey) (_ *routeapi.Route, _ *hostRules, active, ok bool) {
	rules, ok := hi.hostToRoute[host]
	if !ok {
		return nil, nil, false, false
	}
	for _, existing := range rules.active {
		if sameRouteForKey(existing, key) {
			return existing, rules, true, true
		}
	}
	for _, existing := range rules.inactive {
		if sameRouteForKey(existing, key) {
			return existing, rules, false, true
		}
	}
	return nil, rules, false, false
}

func (hi *hostIndex) Remove(route *routeapi.Route) Changes {
	delete(hi.routeToHost, routeKey{namespace: route.Namespace, name: route.Name})
	return hi.remove(route, true, nil)
}

func (hi *hostIndex) remove(route *routeapi.Route, removeLast bool, changes *routeChanges) *routeChanges {
	host := route.Spec.Host
	rules, ok := hi.hostToRoute[host]
	if !ok {
		return nil
	}

	for i, existing := range rules.active {
		if !sameRoute(existing, route) {
			continue
		}
		if changes == nil {
			changes = &routeChanges{}
		}
		rules.removeActive(i, hi.activateFn, changes)
		if removeLast && rules.Empty() {
			delete(hi.hostToRoute, host)
		}
		return changes
	}

	for i, existing := range rules.inactive {
		if !sameRoute(existing, route) {
			continue
		}

		rules.removeInactive(i)
		if removeLast && rules.Empty() {
			delete(hi.hostToRoute, host)
		}
		return nil
	}
	return nil
}

func (hi *hostIndex) Filter(fn func(*routeapi.Route) (keep bool)) Changes {
	changes := &routeChanges{}
	for host, rules := range hi.hostToRoute {
		changed := false
		filtered := rules.active[0:0]
		for _, existing := range rules.active {
			if fn(existing) {
				filtered = append(filtered, existing)
			} else {
				changed = true
				delete(hi.routeToHost, routeKey{namespace: existing.Namespace, name: existing.Name})
			}
		}
		rules.active = filtered

		filtered = rules.inactive[0:0]
		for _, existing := range rules.inactive {
			if fn(existing) {
				filtered = append(filtered, existing)
			} else {
				delete(hi.routeToHost, routeKey{namespace: existing.Namespace, name: existing.Name})
			}
		}
		rules.inactive = filtered

		if rules.Empty() {
			delete(hi.hostToRoute, host)
			continue
		}
		// we only need to filter if the active routes changed
		if !changed {
			continue
		}
		rules.reset(hi.activateFn, changes)
	}
	return changes
}

func (hi *hostIndex) HostLen() int {
	return len(hi.hostToRoute)
}

func (hi *hostIndex) RoutesForHost(host string) ([]*routeapi.Route, bool) {
	rules, ok := hi.hostToRoute[host]
	if !ok {
		return nil, false
	}
	copied := make([]*routeapi.Route, len(rules.active))
	copy(copied, rules.active)
	return copied, true
}

type hostRules struct {
	active   []*routeapi.Route
	inactive []*routeapi.Route
}

func (r *hostRules) Empty() bool {
	return len(r.active) == 0 && len(r.inactive) == 0
}

func (r *hostRules) replace(old, route *routeapi.Route) {
	for i, existing := range r.active {
		if existing == old {
			r.active[i] = route
		}
	}
	for i, existing := range r.inactive {
		if existing == old {
			r.inactive[i] = route
		}
	}
}

func (r *hostRules) add(route *routeapi.Route, fn RouteActivationFunc, changes *routeChanges) {
	if len(r.active) == 0 {
		changes.Activated(route)
		r.active = append(r.active, route)
		return
	}

	active, displaced := fn(changes, r.active, route)
	r.active = active
	if len(displaced) > 0 {
		// if we try to add a route explicitly but it cannot be activated, we should track that.
		for _, existing := range displaced {
			if existing == route {
				changes.Displaced(route)
			}
		}
		r.inactive = append(r.inactive, displaced...)
		sort.Slice(r.inactive, func(i, j int) bool { return routeapi.RouteLessThan(r.inactive[i], r.inactive[j]) })
	}
}

func (r *hostRules) removeActive(i int, fn RouteActivationFunc, changes *routeChanges) {
	r.active = append(r.active[0:i], r.active[i+1:]...)
	// attempt to promote all inactive routes
	if len(r.active) == 0 || i == 0 {
		r.reset(fn, changes)
		return
	}
}

func (r *hostRules) reset(fn RouteActivationFunc, changes *routeChanges) {
	active, displaced := fn(changes, r.active, r.inactive...)
	r.active = active
	r.inactive = displaced
	sort.Slice(r.inactive, func(i, j int) bool { return routeapi.RouteLessThan(r.inactive[i], r.inactive[j]) })
}

func (r *hostRules) removeInactive(i int) {
	r.inactive = append(r.inactive[0:i], r.inactive[i+1:]...)
}

type routeChanges struct {
	active   map[types.UID]*routeapi.Route
	displace map[types.UID]*routeapi.Route
}

func (c *routeChanges) GetActivated() []*routeapi.Route {
	if c == nil {
		return nil
	}
	arr := make([]*routeapi.Route, 0, len(c.active))
	for _, existing := range c.active {
		arr = append(arr, existing)
	}
	return arr
}

func (c *routeChanges) GetDisplaced() []*routeapi.Route {
	if c == nil {
		return nil
	}
	arr := make([]*routeapi.Route, 0, len(c.displace))
	for _, existing := range c.displace {
		arr = append(arr, existing)
	}
	return arr
}

func (c *routeChanges) Activated(route *routeapi.Route) {
	if c.active == nil {
		c.active = make(map[types.UID]*routeapi.Route)
	}
	c.active[route.UID] = route
	delete(c.displace, route.UID)
}
func (c *routeChanges) Displaced(route *routeapi.Route) {
	if c.displace == nil {
		c.displace = make(map[types.UID]*routeapi.Route)
	}
	c.displace[route.UID] = route
	delete(c.active, route.UID)
}
