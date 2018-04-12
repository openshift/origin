package util

import (
	"sort"
	"strings"
)

// sorterFunc is the type of "Less" function that defines map path order.
type sorterFunc func(s1, s2 string) bool

// mapPathSorter sorts a slice of map paths using the sort function.
type mapPathSorter struct {
	data []string
	fn   sorterFunc // closure used in the Less method.
}

// Len returns the length of the data and is part of sort.Interface
func (s *mapPathSorter) Len() int {
	return len(s.data)
}

// Swap swaps the entries for the given indexes and (part of sort.Interface)
func (s *mapPathSorter) Swap(i, j int) {
	s.data[i], s.data[j] = s.data[j], s.data[i]
}

// Less compares two map paths using a closure and is part of sort.Interface
func (s *mapPathSorter) Less(i, j int) bool {
	return s.fn(s.data[i], s.data[j])
}

// sortByGroup sorts the data with any matching prefixes sorted last.
// `reverse` indicates whether or not the data in each group (with and
// without prefixes) needs to be reverse sorted.
// Note that the matching prefixes are sorted based on length.
func sortByGroup(data []string, prefix string, reverse bool) []string {
	patternsAtEnd := func(s1, s2 string) bool {
		if len(prefix) > 0 {
			if strings.HasPrefix(s1, prefix) {
				if !strings.HasPrefix(s2, prefix) {
					return false
				}
			} else if strings.HasPrefix(s2, prefix) {
				return true
			}
		}

		// No prefix or both or neither strings have the prefix.
		if reverse {
			return s1 > s2
		}

		return s1 < s2
	}

	mps := &mapPathSorter{data: data, fn: patternsAtEnd}
	sort.Sort(mps)
	return mps.data
}

// SortMapPaths sorts the data by groups with any matching prefixes sorted at
// the end. The data in each group is reverse sorted.
func SortMapPaths(data []string, prefix string) []string {
	return sortByGroup(data, prefix, true)
}
