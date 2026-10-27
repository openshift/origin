package goconst

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
	"sync"
)

// Issue represents a finding of duplicated strings, numbers, or constants.
// Each Issue includes the position where it was found, how many times it occurs,
// the string itself, and any matching constant name.
// When both test and non-test files are analyzed, OccurrencesCount reflects
// the count within the issue's scope (test or non-test) rather than the global total.
type Issue struct {
	Pos              token.Position
	OccurrencesCount int
	Str              string
	MatchingConst    string
	DuplicateConst   string
	DuplicatePos     token.Position
}

// Config contains all configuration options for the goconst analyzer.
type Config struct {
	// IgnoreStrings is a list of regular expressions to filter strings
	IgnoreStrings []string
	// IgnoreTests indicates whether test files should be excluded
	IgnoreTests bool
	// MatchWithConstants enables matching strings with existing constants
	MatchWithConstants bool
	// MinStringLength is the minimum length a string must have to be reported
	MinStringLength int
	// MinOccurrences is the minimum number of occurrences required to report a string
	MinOccurrences int
	// ParseNumbers enables detection of duplicated numbers
	ParseNumbers bool
	// NumberMin sets the minimum value for reported number matches
	NumberMin int
	// NumberMax sets the maximum value for reported number matches
	NumberMax int
	// ExcludeTypes allows excluding specific types of contexts
	ExcludeTypes map[Type]bool
	// FindDuplicates enables finding constants whose values match existing constants in other packages.
	FindDuplicates bool
	// EvalConstExpressions enables evaluation of constant expressions like Prefix + "suffix"
	EvalConstExpressions bool
	// IgnoreFunctions is a list of function names whose string arguments should be ignored.
	// Supports direct calls (e.g., "println") and one-level qualified calls (e.g., "slog.Info").
	IgnoreFunctions []string
	// IgnoreMapKeys ignores string literals used as keys in map literals
	// (e.g., the "key" in map[string]any{"key": "value"}).
	IgnoreMapKeys bool
}

// NewWithIgnorePatterns creates a new instance of the parser with support for multiple ignore patterns.
// This is an alternative constructor that takes a slice of ignore string patterns.
func NewWithIgnorePatterns(
	path, ignore string,
	ignoreStrings []string,
	ignoreTests, matchConstant, numbers, findDuplicates, evalConstExpressions bool,
	numberMin, numberMax, minLength, minOccurrences int,
	excludeTypes map[Type]bool) *Parser {

	// Join multiple patterns with OR for regex
	var ignoreStringsPattern string
	if len(ignoreStrings) > 0 {
		if len(ignoreStrings) > 1 {
			// Wrap each pattern in parentheses and join with OR
			patterns := make([]string, len(ignoreStrings))
			for i, pattern := range ignoreStrings {
				patterns[i] = "(" + pattern + ")"
			}
			ignoreStringsPattern = strings.Join(patterns, "|")
		} else {
			// Single pattern case
			ignoreStringsPattern = ignoreStrings[0]
		}
	}

	return New(
		path,
		ignore,
		ignoreStringsPattern,
		ignoreTests,
		matchConstant,
		numbers,
		findDuplicates,
		evalConstExpressions,
		numberMin,
		numberMax,
		minLength,
		minOccurrences,
		excludeTypes,
	)
}

// RunWithConfig is a convenience function that runs the analysis with a Config object
// directly supporting multiple ignore patterns.
func RunWithConfig(files []*ast.File, fset *token.FileSet, typeInfo *types.Info, cfg *Config) ([]Issue, error) {
	p := NewWithIgnorePatterns(
		"",
		"",
		cfg.IgnoreStrings,
		cfg.IgnoreTests,
		cfg.MatchWithConstants,
		cfg.ParseNumbers,
		cfg.FindDuplicates,
		cfg.EvalConstExpressions,
		cfg.NumberMin,
		cfg.NumberMax,
		cfg.MinStringLength,
		cfg.MinOccurrences,
		cfg.ExcludeTypes,
	)

	if len(cfg.IgnoreFunctions) > 0 {
		p.SetIgnoreFunctions(cfg.IgnoreFunctions)
	}

	if cfg.IgnoreMapKeys {
		p.SetIgnoreMapKeys(true)
	}

	// Pre-allocate slice based on estimated result size
	expectedIssues := len(files) * 5 // Assuming average of 5 issues per file
	if expectedIssues > 1000 {
		expectedIssues = 1000 // Cap at reasonable maximum
	}

	// Allocate a new buffer
	issueBuffer := make([]Issue, 0, expectedIssues)

	// Process files concurrently
	var wg sync.WaitGroup
	sem := make(chan struct{}, p.maxConcurrency)

	// Create a filtered files slice with capacity hint
	filteredFiles := make([]*ast.File, 0, len(files))

	// Filter test files first if needed
	for _, f := range files {
		if p.ignoreTests {
			if filename := fset.Position(f.Pos()).Filename; strings.HasSuffix(filename, "_test.go") {
				continue
			}
		}
		filteredFiles = append(filteredFiles, f)
	}

	// Process each file in parallel
	for _, f := range filteredFiles {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore

		go func(f *ast.File) {
			defer func() {
				<-sem // release semaphore
				wg.Done()
			}()

			pkgName := ""
			if f.Name != nil {
				pkgName = f.Name.Name
			}

			ast.Walk(&treeVisitor{
				fileSet:     fset,
				packageName: InternString(pkgName),
				p:           p,
				ignoreRegex: p.ignoreStringsRegex,
				typeInfo:    typeInfo,
			}, f)
		}(f)
	}

	wg.Wait()

	p.ProcessResults()

	// Process each string that passed the filters
	p.stringMutex.RLock()
	p.stringCountMutex.RLock()

	// Create a slice to hold the string keys
	stringKeys := make([]string, 0, len(p.strs))

	// Global count is a coarse prefilter; the reporting loop below
	// re-applies minOccurrences per scope (test vs non-test).
	for str := range p.strs {
		if count := p.stringCount[str]; count >= p.minOccurrences {
			stringKeys = append(stringKeys, str)
		}
	}

	sort.Strings(stringKeys)

	// Emit one issue per file where the string appears, so that
	// path-based exclusion can independently filter each one without
	// suppressing legitimate findings in other files.
	// Occurrence counts are scoped: test-file issues report test-file
	// counts and non-test issues report non-test counts, preventing
	// test-file occurrences from inflating production-file reports.
	for _, str := range stringKeys {
		// Copy positions so the sort does not mutate the map value
		// under the read lock.
		positions := append([]ExtendedPos(nil), p.strs[str]...)
		if len(positions) == 0 {
			continue
		}

		sortPositions(positions)

		var nonTestCount, testCount int
		for _, pos := range positions {
			if strings.HasSuffix(pos.Filename, testSuffix) {
				testCount++
			} else {
				nonTestCount++
			}
		}

		// Resolve matching constants per scope so that non-test issues
		// never reference test-only constants (which production code
		// cannot use). Test issues may reference any constant.
		// Only resolve when MatchWithConstants is enabled; FindDuplicates
		// also populates p.consts but should not affect string issues.
		var anyMatchingConst, nonTestMatchingConst string
		if p.matchConstant {
			p.constMutex.RLock()
			raw := p.consts[str]
			p.constMutex.RUnlock()

			if len(raw) > 0 {
				// Copy so the sort does not mutate the map value.
				csts := append([]ConstType(nil), raw...)
				sortConstants(csts)
				anyMatchingConst = csts[0].Name
				for _, cst := range csts {
					if !strings.HasSuffix(cst.Filename, testSuffix) {
						nonTestMatchingConst = cst.Name
						break
					}
				}
			}
		}

		seen := make(map[string]bool)
		for _, pos := range positions {
			if seen[pos.Filename] {
				continue
			}
			seen[pos.Filename] = true

			isTest := strings.HasSuffix(pos.Filename, testSuffix)

			scopeCount := nonTestCount
			if isTest {
				scopeCount = testCount
			}

			if scopeCount < p.minOccurrences {
				continue
			}

			matchingConst := nonTestMatchingConst
			if isTest && matchingConst == "" {
				matchingConst = anyMatchingConst
			}

			issueBuffer = append(issueBuffer, Issue{
				Pos:              pos.Position,
				OccurrencesCount: scopeCount,
				Str:              str,
				MatchingConst:    matchingConst,
			})
		}
	}

	p.stringCountMutex.RUnlock()
	p.stringMutex.RUnlock()

	// Process duplicate constants only when explicitly requested.
	// p.consts may also be populated by matchConstant for constant
	// matching, but those extra entries should not trigger duplicate reports.
	if p.findDuplicates {
		p.constMutex.RLock()

		// Report an issue for every duplicated const within the same scope.
		// Test and non-test constants are compared independently so that a
		// test constant is never flagged as duplicate of a production one.
		for _, group := range duplicateConstGroups(p.consts) {
			allConsts := group.consts

			var nonTestConsts, testConsts []ConstType
			for _, cst := range allConsts {
				if strings.HasSuffix(cst.Filename, testSuffix) {
					testConsts = append(testConsts, cst)
				} else {
					nonTestConsts = append(nonTestConsts, cst)
				}
			}

			for _, scopeConsts := range [][]ConstType{nonTestConsts, testConsts} {
				sortConstants(scopeConsts)
				for i := 1; i < len(scopeConsts); i++ {
					issueBuffer = append(issueBuffer, Issue{
						Pos:            scopeConsts[i].Position,
						Str:            group.displayValue,
						DuplicateConst: scopeConsts[0].Name,
						DuplicatePos:   scopeConsts[0].Position,
					})
				}
			}
		}

		p.constMutex.RUnlock()
	}

	// Don't return the buffer to pool as the caller now owns it
	return issueBuffer, nil
}

// Run analyzes the provided AST files for duplicated strings or numbers
// according to the provided configuration.
// It returns a slice of Issue objects containing the findings.
func Run(files []*ast.File, fset *token.FileSet, typeInfo *types.Info, cfg *Config) ([]Issue, error) {
	return RunWithConfig(files, fset, typeInfo, cfg)
}

func lessPosition(a, b token.Position) bool {
	if a.Filename != b.Filename {
		return a.Filename < b.Filename
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Column < b.Column
}

func sortPositions(positions []ExtendedPos) {
	sort.Slice(positions, func(i, j int) bool {
		return lessPosition(positions[i].Position, positions[j].Position)
	})
}

func sortConstants(consts []ConstType) {
	sort.Slice(consts, func(i, j int) bool {
		return lessPosition(consts[i].Position, consts[j].Position)
	})
}

type duplicateConstGroup struct {
	displayValue string
	consts       []ConstType
}

func duplicateConstGroups(consts Constants) []duplicateConstGroup {
	groups := make(map[string]duplicateConstGroup, len(consts))
	for displayValue, values := range consts {
		for _, cst := range values {
			key := cst.ValueKey()
			if key == "" {
				key = displayValue
			}

			group := groups[key]
			if group.displayValue == "" || displayValue < group.displayValue {
				group.displayValue = displayValue
			}
			group.consts = append(group.consts, cst)
			groups[key] = group
		}
	}

	keys := make([]string, 0, len(groups))
	for key, group := range groups {
		if len(group.consts) > 1 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	result := make([]duplicateConstGroup, 0, len(keys))
	for _, key := range keys {
		result = append(result, groups[key])
	}
	return result
}
