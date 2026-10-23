package pattern

import (
	"regexp"

	"dev.gaijin.team/go/golib/e"
	"dev.gaijin.team/go/golib/fields"
)

// List is a collection of compiled regular expressions.
type List []*regexp.Regexp

// NewList compiles patterns into a List.
// Returns error if any pattern is empty or invalid.
func NewList(patterns ...string) (List, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	list := make(List, 0, len(patterns))

	for _, pattern := range patterns {
		re, err := compilePattern(pattern)
		if err != nil {
			return nil, err
		}

		list = append(list, re)
	}

	return list, nil
}

// MatchFullString returns true if any regex matches the entire string.
// Pattern "test" matches "test" but not "testing" or "contest".
func (l List) MatchFullString(target string) bool {
	for i := range len(l) {
		if matchesFull(l[i], target) {
			return true
		}
	}

	return false
}

// MatchFullStringExcept returns true if any regex matches the entire target
// without also matching the whole of excepted.
//
// A pattern broad enough to select both selects neither of them in particular,
// which is what tells a rule written for one of the two from one that reaches
// it through the other.
func (l List) MatchFullStringExcept(target, excepted string) bool {
	for i := range len(l) {
		if matchesFull(l[i], target) && !matchesFull(l[i], excepted) {
			return true
		}
	}

	return false
}

// matchesFull reports whether re matches the whole of target.
//
// A match spanning [0, len(target)) covers every byte of target, so the matched
// substring is target itself -- no need to allocate it for comparison. Patterns
// match leftmost-longest, so the span of the match found at 0 is the longest
// one there is.
func matchesFull(re *regexp.Regexp, target string) bool {
	loc := re.FindStringIndex(target)

	return loc != nil && loc[0] == 0 && loc[1] == len(target)
}

func compilePattern(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, e.New("empty regular expression is not allowed")
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, e.NewFrom("compile regular expression", err, fields.F("pattern", pattern))
	}

	// Match leftmost-longest. The default is leftmost-first, under which
	// `.*\.(Config|ConfigOption)` commits to the Config branch against
	// "pkg.ConfigOption" and stops short of the end, so testing the span of the
	// match rejects a pattern that does match the whole string. The longest
	// match at position 0 spans the target whenever any match does.
	//
	// The pattern is compiled as the author wrote it. Wrapping it to anchor
	// would put the author's own anchors inside a group and make what a pattern
	// means depend on how it is written.
	re.Longest()

	return re, nil
}
