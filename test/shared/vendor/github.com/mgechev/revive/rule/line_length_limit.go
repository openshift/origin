package rule

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/token"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mgechev/revive/lint"
)

// LineLengthLimitRule lints the number of characters in a line.
type LineLengthLimitRule struct {
	max      int
	excludes []*regexp.Regexp
}

const defaultLineLengthLimit = 80

var _ lint.ConfigurableRule = (*LineLengthLimitRule)(nil)

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *LineLengthLimitRule) Configure(arguments lint.Arguments) error {
	r.max = defaultLineLengthLimit
	r.excludes = nil
	if len(arguments) < 1 {
		return nil
	}

	switch arg := arguments[0].(type) {
	case int64:
		// backward compatibility: if the first argument is an integer, it is treated as the maximum line length.
		return r.setMax(arg)
	case map[string]any:
		return r.configureFromMap(arg)
	default:
		return fmt.Errorf(`invalid argument to the "line-length-limit" rule: expecting an integer or an options map, got %T`, arguments[0])
	}
}

func (r *LineLengthLimitRule) setMax(value int64) error {
	if value < 0 {
		return errors.New(`invalid value passed as argument number to the "line-length-limit" rule`)
	}

	r.max = int(value)
	return nil
}

func (r *LineLengthLimitRule) configureFromMap(options map[string]any) error {
	for k, v := range options {
		switch {
		case isRuleOption(k, "max"):
			maxLength, ok := v.(int64)
			if !ok {
				return fmt.Errorf(`invalid value for the "max" option of the "line-length-limit" rule: expecting an integer, got %T`, v)
			}

			if err := r.setMax(maxLength); err != nil {
				return err
			}
		case isRuleOption(k, "excludes"):
			excludes, err := parseLineLengthExcludes(v)
			if err != nil {
				return err
			}

			r.excludes = excludes
		}
	}

	return nil
}

func parseLineLengthExcludes(value any) ([]*regexp.Regexp, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf(`invalid value for the "excludes" option of the "line-length-limit" rule: expecting a slice of strings, got %T`, value)
	}

	excludes := make([]*regexp.Regexp, 0, len(list))
	for _, v := range list {
		pattern, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf(`invalid value in the "excludes" option of the "line-length-limit" rule: expecting a string, got %T`, v)
		}

		if pattern == "" {
			return nil, errors.New(`invalid value in the "excludes" option of the "line-length-limit" rule: regular expression must not be empty`)
		}

		exp, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf(`invalid value in the "excludes" option of the "line-length-limit" rule: regexp %q does not compile: %w`, pattern, err)
		}

		excludes = append(excludes, exp)
	}

	return excludes, nil
}

// Apply applies the rule to given file.
func (r *LineLengthLimitRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	checker := lintLineLengthNum{
		max:      r.max,
		excludes: r.excludes,
		file:     file,
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
	}

	checker.check()

	return failures
}

// Name returns the rule name.
func (*LineLengthLimitRule) Name() string {
	return "line-length-limit"
}

type lintLineLengthNum struct {
	max       int
	excludes  []*regexp.Regexp
	file      *lint.File
	onFailure func(lint.Failure)
}

func (r lintLineLengthNum) check() {
	f := bytes.NewReader(r.file.Content())
	spaces := strings.Repeat(" ", 4) // tab width = 4
	l := 1
	s := bufio.NewScanner(f)
	for s.Scan() {
		t := s.Text()
		if r.isExcluded(t) {
			l++
			continue
		}

		t = strings.ReplaceAll(t, "\t", spaces)
		c := utf8.RuneCountInString(t)
		if c > r.max {
			r.onFailure(lint.Failure{
				Category: lint.FailureCategoryStyle,
				Position: lint.FailurePosition{
					// Offset not set; it is non-trivial, and doesn't appear to be needed.
					Start: token.Position{
						Filename: r.file.Name,
						Line:     l,
						Column:   0,
					},
					End: token.Position{
						Filename: r.file.Name,
						Line:     l,
						Column:   c,
					},
				},
				Confidence: 1,
				Failure:    fmt.Sprintf("line is %d characters, out of limit %d", c, r.max),
			})
		}
		l++
	}
}

// isExcluded reports whether the raw line matches any of the configured exclude patterns.
func (r lintLineLengthNum) isExcluded(line string) bool {
	for _, exclude := range r.excludes {
		if exclude.MatchString(line) {
			return true
		}
	}

	return false
}
