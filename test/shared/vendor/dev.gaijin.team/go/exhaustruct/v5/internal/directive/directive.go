package directive

import (
	"slices"
	"strings"

	"dev.gaijin.team/go/golib/e"
)

var (
	ErrEmptyDirective      = e.New("empty directive")
	ErrUnknownDirective    = e.New("unknown directive")
	ErrDuplicateDirectives = e.New("duplicate directives")
	ErrDirectiveAfterSpace = e.New("directive after a space is not read")
)

type Directive string

const (
	// Ignore skips checking for a specific struct literal.
	Ignore Directive = "ignore"
	// Enforce forces check even if type is excluded.
	Enforce Directive = "enforce"
	// Optional marks a field as optional.
	Optional Directive = "optional"
)

func (d Directive) IsValid() bool {
	switch d {
	case Ignore, Enforce, Optional:
		return true

	default:
		return false
	}
}

// Directives represents a collection of directives for a single line.
// Multiple directives can be specified comma-separated: //exhaustruct:enforce,optional.
type Directives []Directive

func (ds Directives) Contains(d Directive) bool {
	return slices.Contains(ds, d)
}

// Optionality is what a field's directives say about whether a literal has to
// write the field. These two are all that field metadata reads, so the order a
// comment lists them in, and an ignore written beside them, leave two fields
// annotated alike.
type Optionality struct {
	Optional bool
	Enforced bool
}

// Optionality projects ds onto what field metadata reads. Every consumer of a
// field's directives reads them through this, so none of them can drift from
// the others.
func (ds Directives) Optionality() Optionality {
	return Optionality{
		Optional: ds.Contains(Optional),
		Enforced: ds.Contains(Enforce),
	}
}

// directivePrefix is the exact prefix for exhaustruct directives.
// Format: exhaustruct:<directives> [optional comment], written in either
// comment form.
const directivePrefix = "exhaustruct:"

// commentBody strips the delimiters of either comment form, leaving the text a
// directive may be written in. Both forms carry directives: a line comment
// takes the rest of its line, while a block comment closes, so it can also sit
// ahead of further code or another comment on the same line. In either form the
// directive opens the comment, as //go:build does: a space after the delimiter
// makes the comment prose.
func commentBody(text string) string {
	if body, ok := strings.CutPrefix(text, "//"); ok {
		return body
	}

	if body, ok := strings.CutPrefix(text, "/*"); ok {
		return strings.TrimSuffix(body, "*/")
	}

	return text
}

// prosePrefix is the set of bytes that end a directive list, leaving what
// follows as prose.
const prosePrefix = " \t\n"

func Parse(text string) (found bool, result Directives, errs []error) {
	text, found = strings.CutPrefix(commentBody(text), directivePrefix)
	if !found {
		return false, nil, nil
	}

	// Anything past the first space is prose, not part of the directive.
	list, prose := text, ""
	if idx := strings.IndexAny(text, prosePrefix); idx >= 0 {
		list, prose = text[:idx], text[idx:]
	}

	if list == "" {
		return true, nil, []error{ErrEmptyDirective}
	}

	result, errs = parseParts(list)

	if len(result) == 0 {
		result = nil

		// Separators alone name no directive at all.
		if len(errs) == 0 {
			errs = append(errs, ErrEmptyDirective)
		}
	}

	// A list ending in a separator that prose continues with a directive name
	// is a list the author meant to go on. Read as prose, that name would be
	// dropped in silence.
	if strings.HasSuffix(list, ",") {
		if word := Directive(firstWord(prose)); word.IsValid() {
			errs = append(errs, ErrDirectiveAfterSpace.WithField("directive", word))
		}
	}

	return true, result, errs
}

// firstWord returns the first word of prose, up to a space or a separator.
func firstWord(prose string) string {
	prose = strings.TrimLeft(prose, prosePrefix)

	if idx := strings.IndexAny(prose, prosePrefix+","); idx >= 0 {
		return prose[:idx]
	}

	return prose
}

// parseParts splits the comma-separated body of a directive into directives,
// reporting the ones it does not recognise.
func parseParts(text string) (Directives, []error) {
	parts := strings.Split(text, ",")

	var (
		result  = make(Directives, 0, len(parts))
		errs    []error
		hasDups bool
	)

	for _, part := range parts {
		// A trailing or doubled separator leaves an empty part behind; it names
		// nothing, so there is nothing to report about it.
		if part == "" {
			continue
		}

		d := Directive(part)

		if !d.IsValid() {
			errs = append(errs, ErrUnknownDirective.WithField("directive", d))

			continue
		}

		// giving the resulting size, linear search would be most efficient
		if slices.Contains(result, d) {
			hasDups = true

			continue
		}

		result = append(result, d)
	}

	if hasDups {
		errs = append(errs, ErrDuplicateDirectives)
	}

	return result, errs
}
