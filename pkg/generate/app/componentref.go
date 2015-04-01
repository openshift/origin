package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	imageapi "github.com/openshift/origin/pkg/image/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

// isComponentReference returns true if the provided string appears to be a reference to a source repository
// on disk, at a URL, a docker image name (which might be on a Docker registry or an OpenShift image stream),
// or a template.
func IsComponentReference(s string) bool {
	if len(s) == 0 {
		return false
	}
	all := strings.Split(s, "+")
	_, _, _, err := componentWithSource(all[0])
	return err == nil
}

func componentWithSource(s string) (component, repo string, builder bool, err error) {
	if strings.Contains(s, "~") {
		segs := strings.SplitN(s, "~", 2)
		if len(segs) == 2 {
			builder = true
			switch {
			case len(segs[0]) == 0:
				err = fmt.Errorf("when using '[image]~[code]' form for %q, you must specify a image name", s)
				return
			case len(segs[1]) == 0:
				component = segs[0]
			default:
				component = segs[0]
				repo = segs[1]
			}
		}
	} else {
		component = s
	}
	// TODO: component must be of the form compatible with a pull spec *or* <namespace>/<name>
	if _, err := imageapi.ParseDockerImageReference(component); err != nil {
		return "", "", false, fmt.Errorf("%q is not a valid Docker pull specification: %s", component, err)
	}
	return
}

type ComponentReference interface {
	Input() *ComponentInput
	// Sets Input.Match or returns an error
	Resolve() error
	NeedsSource() bool
}

type ComponentReferences []ComponentReference

func (r ComponentReferences) NeedsSource() (refs ComponentReferences) {
	for _, ref := range r {
		if ref.NeedsSource() {
			refs = append(refs, ref)
		}
	}
	return
}

type GroupedComponentReferences ComponentReferences

func (m GroupedComponentReferences) Len() int      { return len(m) }
func (m GroupedComponentReferences) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m GroupedComponentReferences) Less(i, j int) bool {
	return m[i].Input().Group < m[j].Input().Group
}

func (r ComponentReferences) Group() (refs []ComponentReferences) {
	sorted := make(GroupedComponentReferences, len(r))
	copy(sorted, r)
	sort.Sort(sorted)
	group := -1
	for _, ref := range sorted {
		if ref.Input().Group != group {
			refs = append(refs, ComponentReferences{})
		}
		group = ref.Input().Group
		refs[len(refs)-1] = append(refs[len(refs)-1], ref)
	}
	return
}

type ComponentMatch struct {
	Value       string
	Argument    string
	Name        string
	Description string
	Score       float32

	Builder     bool
	Image       *imageapi.DockerImage
	ImageStream *imageapi.ImageRepository
	ImageTag    string
	Template    *templateapi.Template
}

func (m *ComponentMatch) String() string {
	return m.Argument
}

type Resolver interface {
	// resolvers should return ErrMultipleMatches when more than one result could
	// be construed as a match. Resolvers should set the score to 0.0 if this is a
	// perfect match, and to higher values the less adequate the match is.
	Resolve(value string) (*ComponentMatch, error)
}

type ScoredComponentMatches []*ComponentMatch

func (m ScoredComponentMatches) Len() int           { return len(m) }
func (m ScoredComponentMatches) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m ScoredComponentMatches) Less(i, j int) bool { return m[i].Score < m[j].Score }

func (m ScoredComponentMatches) Exact() []*ComponentMatch {
	out := []*ComponentMatch{}
	for _, match := range m {
		if match.Score == 0.0 {
			out = append(out, match)
		}
	}
	return out
}

type WeightedResolver struct {
	Resolver
	Weight float32
}

// PerfectMatchWeightedResolver returns only matches from resolvers that are identified as exact
// (weight 0.0), and only matches from those resolvers that qualify as exact (score = 0.0). If no
// perfect matches exist, an ErrMultipleMatches is returned indicating the remaining candidate(s).
// Note that this method may resolve ErrMultipleMatches with a single match, indicating an error
// (no perfect match) but with only one candidate.
type PerfectMatchWeightedResolver []WeightedResolver

func (r PerfectMatchWeightedResolver) Resolve(value string) (*ComponentMatch, error) {
	imperfect := []*ComponentMatch{}
	group := []WeightedResolver{}
	for i, resolver := range r {
		if len(group) == 0 || resolver.Weight == group[0].Weight {
			group = append(group, resolver)
			if i != len(r)-1 && r[i+1].Weight == group[0].Weight {
				continue
			}
		}
		match, other, err := resolveExact(WeightedResolvers(group), value)
		switch {
		case match != nil:
			if match.Score == 0.0 {
				return match, nil
			}
			if resolver.Weight != 0.0 {
				match.Score = resolver.Weight * match.Score
			}
			imperfect = append(imperfect, match)
		case len(other) > 0:
			sort.Sort(ScoredComponentMatches(other))
			if other[0].Score == 0.0 && (len(other) == 1 || other[1].Score != 0.0) {
				return other[0], nil
			}
			for _, m := range other {
				if resolver.Weight != 0.0 {
					m.Score = resolver.Weight * m.Score
				}
				imperfect = append(imperfect, m)
			}
		case err != nil:
			return nil, err
		}
		group = nil
	}
	switch len(imperfect) {
	case 0:
		return nil, ErrNoMatch{value: value}
	case 1:
		return imperfect[0], nil
	default:
		return nil, ErrMultipleMatches{value, imperfect}
	}
}

func resolveExact(resolver Resolver, value string) (exact *ComponentMatch, inexact []*ComponentMatch, err error) {
	match, err := resolver.Resolve(value)
	if err != nil {
		switch t := err.(type) {
		case ErrNoMatch:
			return nil, nil, nil
		case ErrMultipleMatches:
			return nil, t.Matches, nil
		default:
			return nil, nil, err
		}
	}
	return match, nil, nil
}

type WeightedResolvers []WeightedResolver

func (r WeightedResolvers) Resolve(value string) (*ComponentMatch, error) {
	candidates := []*ComponentMatch{}
	errs := []error{}
	for _, resolver := range r {
		match, other, err := resolveExact(resolver.Resolver, value)
		switch {
		case match != nil:
			candidates = append(candidates, match)
		case len(other) > 0:
			candidates = append(candidates, other...)
		case err != nil:
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return nil, errors.NewAggregate(errs)
	}
	switch len(candidates) {
	case 0:
		return nil, ErrNoMatch{value: value}
	case 1:
		return candidates[0], nil
	default:
		return nil, ErrMultipleMatches{value, candidates}
	}
}

type ReferenceBuilder struct {
	refs  ComponentReferences
	repos []*SourceRepository
	errs  []error
	group int
}

func (r *ReferenceBuilder) AddImages(inputs []string, fn func(*ComponentInput) ComponentReference) {
	for _, s := range inputs {
		for _, s := range strings.Split(s, "+") {
			input, repo, err := NewComponentInput(s)
			if err != nil {
				r.errs = append(r.errs, err)
				continue
			}
			input.Group = r.group
			ref := fn(input)
			if len(repo) != 0 {
				repository, ok := r.AddSourceRepository(repo)
				if !ok {
					continue
				}
				input.Use(repository)
				repository.UsedBy(ref)
			}
			r.refs = append(r.refs, ref)
		}
		r.group++
	}
}

func (r *ReferenceBuilder) AddGroups(inputs []string) {
	for _, s := range inputs {
		groups := strings.Split(s, "+")
		if len(groups) == 1 {
			r.errs = append(r.errs, fmt.Errorf("group %q only contains a single name", s))
			continue
		}
		to := -1
		for _, group := range groups {
			var match ComponentReference
			for _, ref := range r.refs {
				if group == ref.Input().Value {
					match = ref
					break
				}
			}
			if match == nil {
				r.errs = append(r.errs, fmt.Errorf("the name %q from the group definition is not in use, and can't be used", group))
				break
			}
			if to == -1 {
				to = match.Input().Group
			} else {
				match.Input().Group = to
			}
		}
	}
}

func (r *ReferenceBuilder) AddSourceRepository(input string) (*SourceRepository, bool) {
	for _, existing := range r.repos {
		if input == existing.location {
			return existing, true
		}
	}
	source, err := NewSourceRepository(input)
	if err != nil {
		r.errs = append(r.errs, err)
		return nil, false
	}
	r.repos = append(r.repos, source)
	return source, true
}

func (r *ReferenceBuilder) Result() (ComponentReferences, []*SourceRepository, []error) {
	return r.refs, r.repos, r.errs
}

func NewComponentInput(input string) (*ComponentInput, string, error) {
	// check for image using [image]~ (to indicate builder) or [image]~[code] (builder plus code)
	component, repo, builder, err := componentWithSource(input)
	if err != nil {
		return nil, "", err
	}
	return &ComponentInput{
		From:          input,
		Argument:      input,
		Value:         component,
		ExpectToBuild: builder,
	}, repo, nil
}

type ComponentInput struct {
	Group         int
	From          string
	Argument      string
	Value         string
	ExpectToBuild bool

	Uses  *SourceRepository
	Match *ComponentMatch

	Resolver
}

func (i *ComponentInput) Input() *ComponentInput {
	return i
}

func (i *ComponentInput) NeedsSource() bool {
	return i.ExpectToBuild && i.Uses == nil
}

func (i *ComponentInput) Resolve() error {
	if i.Resolver == nil {
		return ErrNoMatch{value: i.Value, qualifier: "no resolver defined"}
	}
	match, err := i.Resolver.Resolve(i.Value)
	if err != nil {
		return err
	}
	i.Value = match.Value
	i.Argument = match.Argument
	i.Match = match

	return nil
}

func (i *ComponentInput) String() string {
	return i.Value
}

func (i *ComponentInput) Use(repo *SourceRepository) {
	i.Uses = repo
}
