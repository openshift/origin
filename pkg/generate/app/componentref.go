package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/golang/glog"

	imageapi "github.com/openshift/origin/pkg/image/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

// IsComponentReference returns true if the provided string appears to be a reference to a source repository
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

// componentWithSource parses the provided string and returns an image component
// and optionally a repository on success
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
	return
}

// ComponentReference defines an interface for components
type ComponentReference interface {
	// Input contains the input of the component
	Input() *ComponentInput
	// Resolve sets the match in input
	Resolve() error
	// NeedsSource indicates if the component needs source code
	NeedsSource() bool
}

// ComponentReferences is a set of components
type ComponentReferences []ComponentReference

// NeedsSource returns all the components that need source code in order to build
func (r ComponentReferences) NeedsSource() (refs ComponentReferences) {
	for _, ref := range r {
		if ref.NeedsSource() {
			refs = append(refs, ref)
		}
	}
	return
}

func (r ComponentReferences) String() string {
	components := []string{}
	for _, ref := range r {
		components = append(components, ref.Input().Value)
	}

	return strings.Join(components, ",")
}

// GroupedComponentReferences is a set of components that can be grouped
// by their group id
type GroupedComponentReferences ComponentReferences

func (m GroupedComponentReferences) Len() int      { return len(m) }
func (m GroupedComponentReferences) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m GroupedComponentReferences) Less(i, j int) bool {
	return m[i].Input().GroupID < m[j].Input().GroupID
}

// Group groups components based on their group ids
func (r ComponentReferences) Group() (refs []ComponentReferences) {
	sorted := make(GroupedComponentReferences, len(r))
	copy(sorted, r)
	sort.Sort(sorted)
	groupID := -1
	for _, ref := range sorted {
		if ref.Input().GroupID != groupID {
			refs = append(refs, ComponentReferences{})
		}
		groupID = ref.Input().GroupID
		refs[len(refs)-1] = append(refs[len(refs)-1], ref)
	}
	return
}

// ComponentMatch is a match to a provided component
type ComponentMatch struct {
	Value       string
	Argument    string
	Name        string
	Description string
	Score       float32
	Insecure    bool

	Builder     bool
	Image       *imageapi.DockerImage
	ImageStream *imageapi.ImageStream
	ImageTag    string
	Template    *templateapi.Template
}

func (m *ComponentMatch) String() string {
	return m.Argument
}

// IsImage returns whether or not the component match is an
// image or image stream
func (m *ComponentMatch) IsImage() bool {
	return m.Template == nil
}

// IsTemplate returns whether or not the component match is
// a template
func (m *ComponentMatch) IsTemplate() bool {
	return m.Template != nil
}

// Resolver is an interface for resolving provided input to component matches.
// A Resolver should return ErrMultipleMatches when more than one result can
// be constructed as a match. It should also set the score to 0.0 if this is a
// perfect match, and to higher values the less adequate the match is.
type Resolver interface {
	Resolve(value string) (*ComponentMatch, error)
}

// ScoredComponentMatches is a set of component matches grouped by score
type ScoredComponentMatches []*ComponentMatch

func (m ScoredComponentMatches) Len() int           { return len(m) }
func (m ScoredComponentMatches) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m ScoredComponentMatches) Less(i, j int) bool { return m[i].Score < m[j].Score }

// Exact returns all the exact component matches
func (m ScoredComponentMatches) Exact() []*ComponentMatch {
	out := []*ComponentMatch{}
	for _, match := range m {
		if match.Score == 0.0 {
			out = append(out, match)
		}
	}
	return out
}

// WeightedResolver is a resolver identified as exact or not, depending on its weight
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

// Resolve resolves the provided input and returns only exact matches
func (r PerfectMatchWeightedResolver) Resolve(value string) (*ComponentMatch, error) {
	imperfect := ScoredComponentMatches{}
	group := []WeightedResolver{}
	for i, resolver := range r {
		if len(group) == 0 || resolver.Weight == group[0].Weight {
			group = append(group, resolver)
			if i != len(r)-1 && r[i+1].Weight == group[0].Weight {
				continue
			}
		}
		exact, inexact, err := resolveExact(WeightedResolvers(group), value)
		switch {
		case exact != nil:
			if exact.Score == 0.0 {
				return exact, nil
			}
			if resolver.Weight != 0.0 {
				exact.Score = resolver.Weight * exact.Score
			}
			imperfect = append(imperfect, exact)
		case len(inexact) > 0:
			sort.Sort(ScoredComponentMatches(inexact))
			if inexact[0].Score == 0.0 && (len(inexact) == 1 || inexact[1].Score != 0.0) {
				return inexact[0], nil
			}
			for _, m := range inexact {
				if resolver.Weight != 0.0 {
					m.Score = resolver.Weight * m.Score
				}
				imperfect = append(imperfect, m)
			}
		case err != nil:
			glog.V(2).Infof("Error from resolver: %v\n", err)
		}
		group = nil
	}
	switch len(imperfect) {
	case 0:
		return nil, ErrNoMatch{value: value}
	case 1:
		return imperfect[0], nil
	default:
		sort.Sort(imperfect)
		if imperfect[0].Score < imperfect[1].Score {
			return imperfect[0], nil
		}
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

// WeightedResolvers is a set of weighted resolvers
type WeightedResolvers []WeightedResolver

// Resolve resolves the provided input and returns both exact and inexact matches
func (r WeightedResolvers) Resolve(value string) (*ComponentMatch, error) {
	candidates := []*ComponentMatch{}
	errs := []error{}
	for _, resolver := range r {
		exact, inexact, err := resolveExact(resolver.Resolver, value)
		switch {
		case exact != nil:
			candidates = append(candidates, exact)
		case len(inexact) > 0:
			candidates = append(candidates, inexact...)
		case err != nil:
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		glog.V(2).Infof("Errors occurred during resolution: %#v", errs)
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

// ReferenceBuilder is used for building all the necessary object references
// for an application
type ReferenceBuilder struct {
	refs    ComponentReferences
	repos   SourceRepositories
	errs    []error
	groupID int
}

// AddComponents turns all provided component inputs into component references
func (r *ReferenceBuilder) AddComponents(inputs []string, fn func(*ComponentInput) ComponentReference) ComponentReferences {
	refs := ComponentReferences{}
	for _, s := range inputs {
		for _, s := range strings.Split(s, "+") {
			input, repo, err := NewComponentInput(s)
			if err != nil {
				r.errs = append(r.errs, err)
				continue
			}
			input.GroupID = r.groupID
			ref := fn(input)
			if len(repo) != 0 {
				repository, ok := r.AddSourceRepository(repo)
				if !ok {
					continue
				}
				input.Use(repository)
				repository.UsedBy(ref)
			}
			refs = append(refs, ref)
		}
		r.groupID++
	}
	r.refs = append(r.refs, refs...)
	return refs
}

// AddGroups adds group ids to groups of components
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
				to = match.Input().GroupID
			} else {
				match.Input().GroupID = to
			}
		}
	}
}

// AddSourceRepository resolves the input to an actual source repository
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

// Result returns the result of the config conversion to object references
func (r *ReferenceBuilder) Result() (ComponentReferences, SourceRepositories, []error) {
	return r.refs, r.repos, r.errs
}

// NewComponentInput returns a new ComponentInput by checking for image using [image]~
// (to indicate builder) or [image]~[code] (builder plus code)
func NewComponentInput(input string) (*ComponentInput, string, error) {
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

// ComponentInput is the necessary input for creating a component
type ComponentInput struct {
	GroupID       int
	From          string
	Argument      string
	Value         string
	ExpectToBuild bool

	Uses  *SourceRepository
	Match *ComponentMatch

	Resolver
}

// Input returns the component input
func (i *ComponentInput) Input() *ComponentInput {
	return i
}

// NeedsSource indicates if the component input needs source code
func (i *ComponentInput) NeedsSource() bool {
	return i.ExpectToBuild && i.Uses == nil
}

// Resolve sets the match in input
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

// Use adds the provided source repository as the used one
// by the component input
func (i *ComponentInput) Use(repo *SourceRepository) {
	i.Uses = repo
}
