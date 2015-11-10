package parser

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Result represents a test result.
type Result int

// Test result constants
const (
	PASS Result = iota
	FAIL
	SKIP
)

// Report is a collection of package tests.
type Report struct {
	Packages []Package
}

// Package contains the test results of a single package.
type Package struct {
	Name        string
	Time        int
	Tests       []*Test
	CoveragePct string
}

// Test contains the results of a single test.
type Test struct {
	Name   string
	Time   int
	Result Result
	Output []string
}

var (
	regexStatus   = regexp.MustCompile(`^--- (PASS|FAIL|SKIP): (.+) \((\d+\.\d+)(?: seconds|s)\)$`)
	regexCoverage = regexp.MustCompile(`^coverage:\s+(\d+\.\d+)%\s+of\s+statements$`)
	regexResult   = regexp.MustCompile(`^(ok|FAIL)\s+(.+)\s(\d+\.\d+)s(?:\s+coverage:\s+(\d+\.\d+)%\s+of\s+statements)?$`)
)

// Parse parses go test output from reader r and returns a report with the
// results. An optional pkgName can be given, which is used in case a package
// result line is missing.
func Parse(r io.Reader, pkgName string) (*Report, error) {
	reader := bufio.NewReader(r)

	report := &Report{make([]Package, 0)}

	// keep track of tests we find
	var tests []*Test

	// sum of tests' time, use this if current test has no result line (when it is compiled test)
	testsTime := 0

	// current test
	var cur string

	// coverage percentage report for current package
	var coveragePct string

	// parse lines
	for {
		l, _, err := reader.ReadLine()
		if err != nil && err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		line := string(l)

		if strings.HasPrefix(line, "=== RUN ") {
			// new test
			cur = strings.TrimSpace(line[8:])
			tests = append(tests, &Test{
				Name:   cur,
				Result: FAIL,
				Output: make([]string, 0),
			})
		} else if matches := regexResult.FindStringSubmatch(line); len(matches) == 5 {
			if matches[4] != "" {
				coveragePct = matches[4]
			}

			// all tests in this package are finished
			report.Packages = append(report.Packages, Package{
				Name:        matches[2],
				Time:        parseTime(matches[3]),
				Tests:       tests,
				CoveragePct: coveragePct,
			})

			tests = make([]*Test, 0)
			coveragePct = ""
			cur = ""
			testsTime = 0
		} else if matches := regexStatus.FindStringSubmatch(line); len(matches) == 4 {
			cur = matches[2]
			test := findTest(tests, cur)
			if test == nil {
				continue
			}

			// test status
			if matches[1] == "PASS" {
				test.Result = PASS
			} else if matches[1] == "SKIP" {
				test.Result = SKIP
			} else {
				test.Result = FAIL
			}

			test.Name = matches[2]
			testTime := parseTime(matches[3]) * 10
			test.Time = testTime
			testsTime += testTime
		} else if matches := regexCoverage.FindStringSubmatch(line); len(matches) == 2 {
			coveragePct = matches[1]
		} else if strings.HasPrefix(line, "\t") {
			// test output
			test := findTest(tests, cur)
			if test == nil {
				continue
			}
			test.Output = append(test.Output, line[1:])
		}
	}

	if len(tests) > 0 {
		// no result line found
		report.Packages = append(report.Packages, Package{
			Name:        pkgName,
			Time:        testsTime,
			Tests:       tests,
			CoveragePct: coveragePct,
		})
	}

	return report, nil
}

func parseTime(time string) int {
	t, err := strconv.Atoi(strings.Replace(time, ".", "", -1))
	if err != nil {
		return 0
	}
	return t
}

func findTest(tests []*Test, name string) *Test {
	for i := 0; i < len(tests); i++ {
		if tests[i].Name == name {
			return tests[i]
		}
	}
	return nil
}

// Failures counts the number of failed tests in this report
func (r *Report) Failures() int {
	count := 0

	for _, p := range r.Packages {
		for _, t := range p.Tests {
			if t.Result == FAIL {
				count++
			}
		}
	}

	return count
}
