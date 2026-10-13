package adm_upgrade

import (
	"fmt"
	"regexp"
	"strings"
)

// matchRegexp returns nil if the input string matches the pattern.
// If the input string does not match the pattern, it returns an error
// describing the difference.
func matchRegexp(input, pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	if re.MatchString(input) {
		return nil
	}
	patternFragment := pattern

	// the full pattern did not match.  Trim off portions until we find
	// a match, to identify the point of divergence.
	for {
		if len(patternFragment) == 0 {
			break
		}
		patternFragment = patternFragment[:len(patternFragment)-1]

		re, err = regexp.Compile(patternFragment)
		if err != nil {
			continue
		}

		if re.MatchString(input) {
			return fmt.Errorf("expected:\n  %s\nto match regular expression:\n  %s\nbut the longest pattern subset is:\n  %s\nand the unmatched portion of the input is:\n  %s", strings.ReplaceAll(input, "\n", "\n  "), strings.ReplaceAll(pattern, "\n", "\n  "), strings.ReplaceAll(patternFragment, "\n", "\n  "), strings.ReplaceAll(re.ReplaceAllString(input, "<MATCHED>"), "\n", "\n  "))
		}
	}
	return fmt.Errorf("expected:\n  %s\nto match regular expression:\n  %s\nbut no pattern subsets found any matches.", strings.ReplaceAll(input, "\n", "\n  "), strings.ReplaceAll(pattern, "\n", "\n  "))
}
