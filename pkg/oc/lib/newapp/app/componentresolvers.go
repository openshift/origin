package app

import (
	"sort"
	"strings"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/errors"
)

// Resolver is an interface for resolving provided input to component matches.
// A Resolver should return ErrMultipleMatches when more than one result can
// be constructed as a match. It should also set the score to 0.0 if this is a
// perfect match, and to higher values the less adequate the match is.
type Resolver interface {
	Resolve(value string) (*ComponentMatch, error)
}

// Searcher is responsible for performing a search based on the given terms and return
// all results found as component matches. The component match score can be used to
// determine how precise a given match is, where 0.0 is an exact match. All errors
// encountered during search should be returned. The precise flag is a hint to the
// searchers that they may stop searching when they hit their first exact set of
// matches
type Searcher interface {
	Search(precise bool, terms ...string) (ComponentMatches, []error)
	Type() string
}

// WeightedResolver is a resolver identified as exact or not, depending on its weight
type WeightedResolver struct {
	Searcher
	Weight float32
}

// PerfectMatchWeightedResolver returns only matches from resolvers that are identified as exact
// (weight 0.0), and only matches from those resolvers that qualify as exact (score = 0.0). If no
// perfect matches exist, an ErrMultipleMatches is returned indicating the remaining candidate(s).
// Note that this method may resolve ErrMultipleMatches with a single match, indicating an error
// (no perfect match) but with only one candidate.
type PerfectMatchWeightedResolver []WeightedResolver

// Resolve resolves the provided input and returns only exact matches
func (r PerfectMatchWeightedResolver) Resolve(value string) (*ComponentMatch, error) {
	var errs []error
	types := []string{}
	candidates := ScoredComponentMatches{}
	var group MultiSimpleSearcher
	var groupWeight float32 = 0.0
	for i, resolver := range r {
		// lump all resolvers with the same weight into a single group
		if len(group) == 0 || resolver.Weight == groupWeight {
			group = append(group, resolver.Searcher)
			groupWeight = resolver.Weight
			if i != len(r)-1 && r[i+1].Weight == groupWeight {
				continue
			}
		}
		matches, err := group.Search(true, value)
		if err != nil {
			glog.V(5).Infof("Error from resolver: %v\n", err)
			errs = append(errs, err...)
		}
		types = append(types, group.Type())

		sort.Sort(ScoredComponentMatches(matches))
		if len(matches) > 0 && matches[0].Score == 0.0 && (len(matches) == 1 || matches[1].Score != 0.0) {
			return matches[0], errors.NewAggregate(errs)
		}
		for _, m := range matches {
			if groupWeight != 0.0 {
				m.Score = groupWeight * m.Score
			}
			candidates = append(candidates, m)
		}
		group = nil
	}

	switch len(candidates) {
	case 0:
		return nil, ErrNoMatch{Value: value, Errs: errs, Type: strings.Join(types, ", ")}
	case 1:
		if candidates[0].Score != 0.0 {
			if candidates[0].NoTagsFound {
				return nil, ErrNoTagsFound{Value: value, Match: candidates[0], Errs: errs}
			}
			return nil, ErrPartialMatch{Value: value, Match: candidates[0], Errs: errs}
		}
		return candidates[0], errors.NewAggregate(errs)
	default:
		sort.Sort(candidates)
		if candidates[0].Score < candidates[1].Score {
			if candidates[0].Score != 0.0 {
				return nil, ErrPartialMatch{Value: value, Match: candidates[0], Errs: errs}
			}
			return candidates[0], errors.NewAggregate(errs)
		}
		return nil, ErrMultipleMatches{Value: value, Matches: candidates, Errs: errs}
	}
}

// FirstMatchResolver simply takes the first search result returned by the
// searcher it holds and resolves it to that match. An ErrMultipleMatches will
// never happen given it will just take the first result, but a ErrNoMatch can
// happen if the searcher returns no matches.
type FirstMatchResolver struct {
	Searcher Searcher
}

// Resolve resolves as the first match returned by the Searcher
func (r FirstMatchResolver) Resolve(value string) (*ComponentMatch, error) {
	matches, err := r.Searcher.Search(true, value)
	if len(matches) == 0 {
		return nil, ErrNoMatch{Value: value, Errs: err, Type: r.Searcher.Type()}
	}
	return matches[0], errors.NewAggregate(err)
}

// HighestScoreResolver takes search result returned by the searcher it holds
// and resolves it to the highest scored match present. An ErrMultipleMatches
// will never happen given it will just take the best scored result, but a
// ErrNoMatch can happen if the searcher returns no matches.
type HighestScoreResolver struct {
	Searcher Searcher
}

// Resolve resolves as the first highest scored match returned by the Searcher
func (r HighestScoreResolver) Resolve(value string) (*ComponentMatch, error) {
	matches, err := r.Searcher.Search(true, value)
	if len(matches) == 0 {
		return nil, ErrNoMatch{Value: value, Errs: err, Type: r.Searcher.Type()}
	}
	sort.Sort(ScoredComponentMatches(matches))
	return matches[0], errors.NewAggregate(err)
}

// HighestUniqueScoreResolver takes search result returned by the searcher it
// holds and resolves it to the highest scored match present. If more than one
// match exists with that same score, returns an ErrMultipleMatches. A ErrNoMatch
// can happen if the searcher returns no matches.
type HighestUniqueScoreResolver struct {
	Searcher Searcher
}

// Resolve resolves as the highest scored match returned by the Searcher, and
// guarantees the match is unique (the only match with that given score)
func (r HighestUniqueScoreResolver) Resolve(value string) (*ComponentMatch, error) {
	matches, err := r.Searcher.Search(true, value)
	sort.Sort(ScoredComponentMatches(matches))
	switch len(matches) {
	case 0:
		return nil, ErrNoMatch{Value: value, Errs: err, Type: r.Searcher.Type()}
	case 1:
		return matches[0], errors.NewAggregate(err)
	default:
		if matches[0].Score == matches[1].Score {
			equal := ComponentMatches{}
			for _, m := range matches {
				if m.Score != matches[0].Score {
					break
				}
				equal = append(equal, m)
			}
			return nil, ErrMultipleMatches{Value: value, Matches: equal, Errs: err}
		}
		return matches[0], errors.NewAggregate(err)
	}
}

// UniqueExactOrInexactMatchResolver takes search result returned by the searcher
// it holds. Returns the single exact match present, if more that one exact match
// is present, returns a ErrMultipleMatches. If no exact match is present, try with
// inexact ones, which must also be unique otherwise ErrMultipleMatches. A ErrNoMatch
// can happen if the searcher returns no exact or inexact matches.
type UniqueExactOrInexactMatchResolver struct {
	Searcher Searcher
}

// Resolve resolves as the single exact or inexact match present.
func (r UniqueExactOrInexactMatchResolver) Resolve(value string) (*ComponentMatch, error) {
	matches, err := r.Searcher.Search(true, value)
	sort.Sort(ScoredComponentMatches(matches))

	exact := matches.Exact()
	switch len(exact) {
	case 0:
		inexact := matches.Inexact()
		switch len(inexact) {
		case 0:
			return nil, ErrNoMatch{Value: value, Errs: err, Type: r.Searcher.Type()}
		case 1:
			return inexact[0], errors.NewAggregate(err)
		default:
			return nil, ErrMultipleMatches{Value: value, Matches: matches, Errs: err}
		}
	case 1:
		return exact[0], errors.NewAggregate(err)
	default:
		return nil, ErrMultipleMatches{Value: value, Matches: matches, Errs: err}
	}
}

// PipelineResolver returns a dummy ComponentMatch for any value input.  It is
// used to provide a dummy component for for the pipeline/Jenkinsfile strategy.
type PipelineResolver struct {
}

// Resolve returns a dummy ComponentMatch for any value input.
func (r PipelineResolver) Resolve(value string) (*ComponentMatch, error) {
	return &ComponentMatch{
		Value:     value,
		LocalOnly: true,
	}, nil
}

// MultiSimpleSearcher is a set of searchers
type MultiSimpleSearcher []Searcher

func (s MultiSimpleSearcher) Type() string {
	t := []string{}
	for _, searcher := range s {
		t = append(t, searcher.Type())
	}
	return strings.Join(t, ", ")
}

// Search searches using all searchers it holds
func (s MultiSimpleSearcher) Search(precise bool, terms ...string) (ComponentMatches, []error) {
	var errs []error
	componentMatches := ComponentMatches{}
	for _, searcher := range s {
		matches, err := searcher.Search(precise, terms...)
		if len(err) > 0 {
			errs = append(errs, err...)
			continue
		}
		componentMatches = append(componentMatches, matches...)
	}
	sort.Sort(ScoredComponentMatches(componentMatches))
	return componentMatches, errs
}

// WeightedSearcher is a searcher identified as exact or not, depending on its weight
type WeightedSearcher struct {
	Searcher
	Weight float32
}

// MultiWeightedSearcher is a set of weighted searchers where lower weight has higher
// priority in search results
type MultiWeightedSearcher []WeightedSearcher

func (s MultiWeightedSearcher) Type() string {
	t := []string{}
	for _, searcher := range s {
		t = append(t, searcher.Type())
	}
	return strings.Join(t, ", ")
}

// Search searches using all searchers it holds and score according to searcher height
func (s MultiWeightedSearcher) Search(precise bool, terms ...string) (ComponentMatches, []error) {
	componentMatches := ComponentMatches{}
	var errs []error
	for _, searcher := range s {
		matches, err := searcher.Search(precise, terms...)
		if len(err) > 0 {
			errs = append(errs, err...)
			continue
		}
		for _, match := range matches {
			match.Score += searcher.Weight
			componentMatches = append(componentMatches, match)
		}
	}
	sort.Sort(ScoredComponentMatches(componentMatches))
	return componentMatches, errs
}
