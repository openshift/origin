package ginkgo

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"github.com/openshift/origin/pkg/version"
)

func writeJUnitReport(filePrefix, name string, tests []*testCase, dir string, duration time.Duration, errOut io.Writer, additionalResults ...*junitapi.JUnitTestCase) error {
	s := &junitapi.JUnitTestSuite{
		Name:     name,
		Duration: duration.Seconds(),
		Properties: []*junitapi.TestSuiteProperty{
			{
				Name:  "TestVersion",
				Value: version.Get().String(),
			},
		},
	}
	for _, test := range tests {
		switch {
		case test.skipped:
			s.NumTests++
			s.NumSkipped++
			s.TestCases = append(s.TestCases, &junitapi.JUnitTestCase{
				Name:      test.name,
				SystemOut: string(test.out),
				Duration:  test.duration.Seconds(),
				SkipMessage: &junitapi.SkipMessage{
					Message: lastLinesUntil(string(test.out), 100, "skip ["),
				},
			})
		case test.failed:
			s.NumTests++
			s.NumFailed++
			s.TestCases = append(s.TestCases, &junitapi.JUnitTestCase{
				Name:      test.name,
				SystemOut: string(test.out),
				Duration:  test.duration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: lastLinesUntil(string(test.out), 100, "fail ["),
				},
			})
		case test.success:
			if test.flake {
				s.NumTests++
				s.NumFailed++
				s.TestCases = append(s.TestCases, &junitapi.JUnitTestCase{
					Name:      test.name,
					SystemOut: string(test.out),
					Duration:  test.duration.Seconds(),
					FailureOutput: &junitapi.FailureOutput{
						Output: lastLinesUntil(string(test.out), 100, "flake:"),
					},
				})
			}
			s.NumTests++
			s.TestCases = append(s.TestCases, &junitapi.JUnitTestCase{
				Name:     test.name,
				Duration: test.duration.Seconds(),
			})
		}
	}
	for _, result := range additionalResults {
		switch {
		case result.SkipMessage != nil:
			s.NumSkipped++
		case result.FailureOutput != nil:
			s.NumFailed++
		}
		s.NumTests++
		s.TestCases = append(s.TestCases, result)
	}
	out, err := xml.Marshal(s)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s_%s.xml", filePrefix, time.Now().UTC().Format("20060102-150405")))
	fmt.Fprintf(errOut, "Writing JUnit report to %s\n\n", path)
	return ioutil.WriteFile(path, out, 0640)
}

func lastLinesUntil(output string, max int, until ...string) string {
	output = strings.TrimSpace(output)
	index := len(output) - 1
	if index < 0 || max == 0 {
		return output
	}
	for max > 0 {
		next := strings.LastIndex(output[:index], "\n")
		if next <= 0 {
			return strings.TrimSpace(output)
		}
		// empty lines don't count
		line := strings.TrimSpace(output[next+1 : index])
		if len(line) > 0 {
			max--
		}
		index = next
		if stringStartsWithAny(line, until) {
			break
		}
	}
	return strings.TrimSpace(output[index:])
}

func stringStartsWithAny(s string, contains []string) bool {
	for _, match := range contains {
		if strings.HasPrefix(s, match) {
			return true
		}
	}
	return false
}
