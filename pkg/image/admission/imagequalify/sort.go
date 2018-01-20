package imagequalify

import (
	"sort"

	"github.com/openshift/origin/pkg/image/admission/imagequalify/api"
)

type lessFunc func(x, y *api.ImageQualifyRule) bool

type multiSorter struct {
	rules []api.ImageQualifyRule
	less  []lessFunc
}

var _ sort.Interface = &multiSorter{}

// Sort sorts the argument slice according to the comparator functions
// passed to orderBy.
func (s *multiSorter) Sort(rules []api.ImageQualifyRule) {
	s.rules = rules
	sort.Sort(s)
}

// orderBy returns a Sorter that sorts using a number of comparator
// functions.
func orderBy(less ...lessFunc) *multiSorter {
	return &multiSorter{
		less: less,
	}
}

// Len is part of sort.Interface.
func (s *multiSorter) Len() int {
	return len(s.rules)
}

// Swap is part of sort.Interface.
func (s *multiSorter) Swap(i, j int) {
	s.rules[i], s.rules[j] = s.rules[j], s.rules[i]
}

// Less is part of sort.Interface.
func (s *multiSorter) Less(i, j int) bool {
	p, q := s.rules[i], s.rules[j]
	// Try all but the last comparison.
	var k int
	for k = 0; k < len(s.less)-1; k++ {
		less := s.less[k]
		switch {
		case less(&p, &q):
			// p < q, so we have a decision.
			return true
		case less(&q, &p):
			// p > q, so we have a decision.
			return false
		}
		// p == q; try the next comparison.
	}

	return s.less[k](&p, &q)
}
