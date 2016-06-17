package flags

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"

	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/validation/field"
)

// Apply stores the provided arguments onto a flag set, reporting any errors
// encountered during the process.
func Apply(args map[string][]string, flags *pflag.FlagSet) []error {
	var errs []error
	for key, value := range args {
		flag := flags.Lookup(key)
		if flag == nil {
			errs = append(errs, field.Invalid(field.NewPath("flag"), key, "is not a valid flag"))
			continue
		}
		for _, s := range value {
			if err := flag.Value.Set(s); err != nil {
				errs = append(errs, field.Invalid(field.NewPath(key), s, fmt.Sprintf("could not be set: %v", err)))
				break
			}
		}
	}
	return errs
}

func Resolve(args map[string][]string, fn func(*pflag.FlagSet)) []error {
	fs := pflag.NewFlagSet("extended", pflag.ContinueOnError)
	fn(fs)
	return Apply(args, fs)
}

// ComponentFlag represents a set of enabled components used in a command line.
type ComponentFlag struct {
	enabled    string
	disabled   string
	enabledSet func() bool

	calculated sets.String

	allowed  sets.String
	mappings map[string][]string
}

// NewComponentFlag returns a flag that represents the allowed components and can be bound to command line flags.
func NewComponentFlag(mappings map[string][]string, allowed ...string) *ComponentFlag {
	set := sets.NewString(allowed...)
	return &ComponentFlag{
		allowed:    set,
		mappings:   mappings,
		enabled:    strings.Join(set.List(), ","),
		enabledSet: func() bool { return false },
	}
}

// DefaultEnable resets the enabled components to only those provided that are also in the allowed
// list.
func (f *ComponentFlag) DefaultEnable(components ...string) *ComponentFlag {
	f.enabled = strings.Join(f.allowed.Union(sets.NewString(components...)).List(), ",")
	return f
}

// DefaultDisable resets the default enabled set to all allowed components except the provided.
func (f *ComponentFlag) DefaultDisable(components ...string) *ComponentFlag {
	f.enabled = strings.Join(f.allowed.Difference(sets.NewString(components...)).List(), ",")
	return f
}

// Disable marks the provided components as disabled.
func (f *ComponentFlag) Disable(components ...string) {
	f.Calculated().Delete(components...)
}

// Enabled returns true if the component is enabled.
func (f *ComponentFlag) Enabled(name string) bool {
	return f.Calculated().Has(name)
}

// Calculated returns the effective enabled list.
func (f *ComponentFlag) Calculated() sets.String {
	if f.calculated == nil {
		f.calculated = f.Expand(f.enabled).Difference(f.Expand(f.disabled)).Intersection(f.allowed)
	}
	return f.calculated
}

// Validate returns a copy of the set of enabled components, or an error if there are conflicts.
func (f *ComponentFlag) Validate() (sets.String, error) {
	enabled := f.Expand(f.enabled)
	disabled := f.Expand(f.disabled)
	if diff := enabled.Difference(f.allowed); enabled.Len() > 0 && diff.Len() > 0 {
		return nil, fmt.Errorf("the following components are not recognized: %s", strings.Join(diff.List(), ", "))
	}
	if diff := disabled.Difference(f.allowed); disabled.Len() > 0 && diff.Len() > 0 {
		return nil, fmt.Errorf("the following components are not recognized: %s", strings.Join(diff.List(), ", "))
	}
	if inter := enabled.Intersection(disabled); f.enabledSet() && inter.Len() > 0 {
		return nil, fmt.Errorf("the following components can't be both disabled and enabled: %s", strings.Join(inter.List(), ", "))
	}
	return enabled.Difference(disabled), nil
}

// Expand turns a string into a fully expanded set of components, resolving any mappings.
func (f *ComponentFlag) Expand(value string) sets.String {
	if len(value) == 0 {
		return sets.NewString()
	}
	items := strings.Split(value, ",")
	set := sets.NewString()
	for _, s := range items {
		if mapped, ok := f.mappings[s]; ok {
			set.Insert(mapped...)
		} else {
			set.Insert(s)
		}
	}
	return set
}

// Allowed returns a copy of the allowed list of components.
func (f *ComponentFlag) Allowed() sets.String {
	return sets.NewString(f.allowed.List()...)
}

// Mappings returns a copy of the mapping list for short names.
func (f *ComponentFlag) Mappings() map[string][]string {
	copied := make(map[string][]string)
	for k, v := range f.mappings {
		copiedV := make([]string, len(v))
		copy(copiedV, v)
		copied[k] = copiedV
	}
	return copied
}

// Bind registers the necessary flags with a flag set.
func (f *ComponentFlag) Bind(flags *pflag.FlagSet, flagFormat, messagePrefix string) {
	flags.StringVar(&f.enabled, fmt.Sprintf(flagFormat, "enable"), f.enabled, messagePrefix+" enable")
	flags.StringVar(&f.disabled, fmt.Sprintf(flagFormat, "disable"), f.disabled, messagePrefix+" disable")

	f.enabledSet = func() bool { return flags.Lookup(fmt.Sprintf(flagFormat, "enable")).Changed }
}
