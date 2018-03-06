package dockerhelper

import (
	"net"
	"sort"
)

// ipv4 represented as an unsigned 32-bit integer
type ipv4 uint32

// IPV4Range represents a range of ipv4s
type IPV4Range struct {
	from ipv4
	to   ipv4
}

// Contains returns whether an IPV4Range is contained within another IPV4Range
func (r IPV4Range) Contains(aRange IPV4Range) bool {
	return aRange.from >= r.from && aRange.to <= r.to
}

// IPV4RangeList represents a collection of IPV4Ranges
type IPV4RangeList []IPV4Range

// Functions needed to support sorting on IPV4RangeList

// Len returns the length of entries within an IPV4RangeList
func (l IPV4RangeList) Len() int { return len(l) }

// Swap swaps entries within IPV4RangeList at indices i, and j
func (l IPV4RangeList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

// Less returns whether a starting ipv4 at index i within this list, is less than a starting
// ipv4 at index j within this list
func (l IPV4RangeList) Less(i, j int) bool { return l[i].from < l[j].from }

// Contains returns true if the union of the ranges in this
// list completely contains the given range
func (l IPV4RangeList) Contains(r IPV4Range) bool {
	// Get a compact version of the list
	c := compactRangeList(l)
	for _, e := range c {
		if e.Contains(r) {
			return true
		}
	}
	return false
}

// compactRangeList will return a list of ranges that has
// been compacted by joining overlapping or adjacent ranges.
// For example, if I have ranges 1-2,3-5,4-8. This will compact
// all of them to a single range 1-8.
// It makes a copy of the original list, leaving it intact
func compactRangeList(list IPV4RangeList) IPV4RangeList {
	if len(list) == 0 {
		return list
	}
	// Make a sorted list by the range lower bounds
	sorted := make(IPV4RangeList, len(list))
	copy(sorted, list)
	sort.Sort(sorted)

	// Make a second list with merged ranges
	result := IPV4RangeList{}
	var current = sorted[0]
	for i := 1; i < len(sorted); i++ {
		// If current range's upper bound is adjacent to or
		// overlapping the next one's lower bound then
		// simply extend the current one
		if current.to+1 >= sorted[i].from {
			if sorted[i].to > current.to {
				current.to = sorted[i].to
			}
		} else {
			result = append(result, current)
			current = sorted[i]
		}
	}
	result = append(result, current)
	return result
}

// toIPv4 casts an ip of type IP to our ipv4 representation
func toIPv4(ip net.IP) ipv4 {
	ip = ip.To4()
	if len(ip) < 4 {
		return 0
	}
	return ipv4(ip[0])<<24 |
		ipv4(ip[1])<<16 |
		ipv4(ip[2])<<8 |
		ipv4(ip[3])
}

// fromCIDR creates a new IPV4Range from a given IPNet
func fromCIDR(ipnet *net.IPNet) IPV4Range {
	// The lower bound of the range is
	// always going to be the IP associated with
	// the IPNet
	lower := toIPv4(ipnet.IP)

	// The upper bound can be calculated by adding
	// the inverse of the mask to the lower bound.
	ones, _ := ipnet.Mask.Size()
	toAdd := ^uint32(0) >> uint32(ones)
	upper := uint32(lower) + uint32(toAdd)
	return IPV4Range{lower, ipv4(upper)}
}
