package analyzer

import (
	"flag"
	"strings"

	"dev.gaijin.team/go/exhaustruct/v5/internal/pattern"
)

type Config struct {
	// EnforcePatterns is a list of regular expressions to match type names that
	// should be checked. Anonymous structs can be matched by '<anonymous>' alias.
	//
	// A type pattern selects the types checked under ExplicitMode; without it
	// every type is checked already, so only field patterns change anything.
	//
	// A pattern may name a field as 'Type#Field', which requires that field
	// inside a type already being checked, in either mode. A pattern matching
	// the field and not the type holding it outranks what the type says; one
	// broad enough to match both names no field in particular.
	//
	// Each regular expression must match the full type name, including package path.
	// For example, to match type `net/http.Cookie` regular expression should be
	// `.*/http\.Cookie`, but not `http\.Cookie`.
	EnforcePatterns Patterns //exhaustruct:optional

	// IgnorePatterns is a list of regular expressions to match type names that
	// should be skipped from checking. Anonymous structs can be matched by
	// '<anonymous>' alias.
	//
	// Has precedence over EnforcePatterns.
	//
	// Each regular expression must match the full type name, including package path.
	// For example, to match type `net/http.Cookie` regular expression should be
	// `.*/http\.Cookie`, but not `http\.Cookie`.
	IgnorePatterns Patterns //exhaustruct:optional

	// OptionalPatterns is a list of regular expressions to match type names where
	// all fields are treated as optional. Anonymous structs can be matched by
	// '<anonymous>' alias.
	//
	// Each regular expression must match the full type name, including package path.
	// For example, to match type `net/http.Cookie` regular expression should be
	// `.*/http\.Cookie`, but not `http\.Cookie`.
	OptionalPatterns Patterns //exhaustruct:optional

	// AllowEmpty allows empty structures, effectively excluding them from the check.
	AllowEmpty bool //exhaustruct:optional

	// AllowEmptyPatterns is a list of regular expressions to match type names that
	// should be allowed to be empty. Anonymous structs can be matched by
	// '<anonymous>' alias.
	//
	// Each regular expression must match the full type name, including package path.
	// For example, to match type `net/http.Cookie` regular expression should be
	// `.*/http\.Cookie`, but not `http\.Cookie`.
	AllowEmptyPatterns Patterns //exhaustruct:optional

	// AllowEmptyReturns allows empty structures in return statements.
	AllowEmptyReturns bool //exhaustruct:optional

	// AllowEmptyDeclarations allows empty structures in variable declarations.
	AllowEmptyDeclarations bool //exhaustruct:optional

	// AllowEmptyBlankAssignments allows empty structures the blank identifier
	// receives, in a declaration or an assignment. `var _ Iface = T{}`, which
	// asserts at compile time that T implements Iface, is written this way.
	AllowEmptyBlankAssignments bool //exhaustruct:optional

	// ReportFullTypePath enables full package path in error messages instead of
	// short package name. This helps when configuring include/exclude patterns,
	// as import aliases can make short names ambiguous.
	ReportFullTypePath bool //exhaustruct:optional

	// ExplicitMode enables opt-in checking. When true, only types marked with
	// //exhaustruct:enforce directive or matching enforce-rx patterns are checked.
	ExplicitMode bool //exhaustruct:optional
}

// bindToFlagSet binds the config fields to the provided flag set.
func (c *Config) bindToFlagSet(fs *flag.FlagSet) {
	fs.BoolVar(&c.ExplicitMode, "explicit", c.ExplicitMode,
		"Enable explicit mode: only check types marked with //exhaustruct:enforce "+
			"directive or matching -enforce-rx patterns")

	fs.Var(&c.EnforcePatterns, "enforce-rx",
		"Regular expression to match type names that should be checked. "+
			"Selects the types checked under -explicit. A pattern may name a field "+
			"as Type#Field, which requires that field in either mode; one matching "+
			"the field and not the type holding it outranks what the type says. "+
			"Anonymous structs can be matched by '<anonymous>' alias. "+
			"Each regex must match the full type name including package path. "+
			"Example: `.*/http\\.Cookie`. Can be used multiple times.")

	fs.Var(&c.IgnorePatterns, "ignore-rx",
		"Regular expression to skip type names from checking, has precedence over -enforce-rx. "+
			"Anonymous structs can be matched by '<anonymous>' alias. "+
			"Each regex must match the full type name including package path. "+
			"Example: `.*/http\\.Cookie`. Can be used multiple times.")

	fs.Var(&c.OptionalPatterns, "optional-rx",
		"Regular expression to match type names where all fields are optional. "+
			"Anonymous structs can be matched by '<anonymous>' alias. "+
			"Each regex must match the full type name including package path. "+
			"Example: `.*/http\\.Cookie`. Can be used multiple times.")

	fs.BoolVar(&c.AllowEmpty, "allow-empty", c.AllowEmpty,
		"Allow empty structures, effectively excluding them from the check")

	fs.Var(&c.AllowEmptyPatterns, "allow-empty-rx",
		"Regular expression to match type names that should be allowed to be empty. "+
			"Anonymous structs can be matched by '<anonymous>' alias. "+
			"Each regex must match the full type name including package path. "+
			"Example: `.*/http\\.Cookie`. Can be used multiple times.")

	fs.BoolVar(&c.AllowEmptyReturns, "allow-empty-returns", c.AllowEmptyReturns,
		"Allow empty structures in return statements")

	fs.BoolVar(&c.AllowEmptyDeclarations, "allow-empty-declarations", c.AllowEmptyDeclarations,
		"Allow empty structures in variable declarations")

	fs.BoolVar(&c.AllowEmptyBlankAssignments, "allow-empty-blank-assignments", c.AllowEmptyBlankAssignments,
		"Allow empty structures the blank identifier receives, as in 'var _ Iface = T{}'")

	fs.BoolVar(&c.ReportFullTypePath, "report-full-type-path", c.ReportFullTypePath,
		"Report full package path in error messages (e.g., 'net/http.Cookie' instead of 'http.Cookie'). "+
			"Useful for identifying types when configuring enforce/ignore patterns.")
}

// Patterns is a list of regular expression patterns. It implements
// flag.Value, validating each value as a regular expression at
// flag-parse time so invalid patterns fail early.
type Patterns []string

// String returns the patterns joined with commas (flag.Value interface).
func (p *Patterns) String() string {
	if p == nil {
		return ""
	}

	return strings.Join(*p, ",")
}

// Set validates and appends a pattern (flag.Value interface).
func (p *Patterns) Set(value string) error {
	if _, err := pattern.NewList(value); err != nil {
		return err //nolint:wrapcheck
	}

	*p = append(*p, value)

	return nil
}
