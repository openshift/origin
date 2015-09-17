package app

import (
	"fmt"
	"sort"
	"strings"
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
	// Search sets the search matches in input
	Search() error
	// NeedsSource indicates if the component needs source code
	NeedsSource() bool
}

// ComponentReferences is a set of components
type ComponentReferences []ComponentReference

func (r ComponentReferences) filter(filterFunc func(ref ComponentReference) bool) ComponentReferences {
	refs := ComponentReferences{}
	for _, ref := range r {
		if filterFunc(ref) {
			refs = append(refs, ref)
		}
	}
	return refs
}

// NeedsSource returns all the components that need source code in order to build
func (r ComponentReferences) NeedsSource() (refs ComponentReferences) {
	return r.filter(func(ref ComponentReference) bool {
		return ref.NeedsSource()
	})
}

// ImageComponentRefs returns the list of component references to images
func (r ComponentReferences) ImageComponentRefs() (refs ComponentReferences) {
	return r.filter(func(ref ComponentReference) bool {
		return ref.Input() != nil && ref.Input().ResolvedMatch != nil && ref.Input().ResolvedMatch.IsImage()
	})
}

// TemplateComponentRefs returns the list of component references to templates
func (r ComponentReferences) TemplateComponentRefs() (refs ComponentReferences) {
	return r.filter(func(ref ComponentReference) bool {
		return ref.Input() != nil && ref.Input().ResolvedMatch != nil && ref.Input().ResolvedMatch.IsTemplate()
	})
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

func (r *ReferenceBuilder) AddExistingSourceRepository(source *SourceRepository) {
	r.repos = append(r.repos, source)
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

	Uses          *SourceRepository
	ResolvedMatch *ComponentMatch
	SearchMatches ComponentMatches

	Resolver
	Searcher
}

// Input returns the component input
func (i *ComponentInput) Input() *ComponentInput {
	return i
}

// NeedsSource indicates if the component input needs source code
func (i *ComponentInput) NeedsSource() bool {
	return i.ExpectToBuild && i.Uses == nil
}

// Resolve sets the unique match in input
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
	i.ResolvedMatch = match
	return nil
}

// Search sets the search matches in input
func (i *ComponentInput) Search() error {
	if i.Searcher == nil {
		return ErrNoMatch{value: i.Value, qualifier: "no searcher defined"}
	}
	matches, err := i.Searcher.Search(i.Value)
	if matches != nil {
		i.SearchMatches = matches
	}
	return err
}

func (i *ComponentInput) String() string {
	return i.Value
}

// Use adds the provided source repository as the used one
// by the component input
func (i *ComponentInput) Use(repo *SourceRepository) {
	i.Uses = repo
}
