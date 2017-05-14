package gotest

import (
	"bufio"
	"os"
	"reflect"
	"testing"

	"github.com/openshift/origin/tools/junitreport/pkg/api"
	"github.com/openshift/origin/tools/junitreport/pkg/builder/flat"
)

// TestFlatParse tests that parsing the `go test` output in the test directory with a flat builder works as expected
func TestFlatParse(t *testing.T) {
	var testCases = []struct {
		name           string
		testFile       string
		expectedSuites *api.TestSuites
	}{
		{
			name:     "basic",
			testFile: "1.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:     "package/name",
						NumTests: 2,
						Duration: 0.16,
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.06,
							},
							{
								Name:     "TestTwo",
								Duration: 0.1,
							},
						},
					},
				},
			},
		},
		{
			name:     "failure",
			testFile: "2.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:      "package/name",
						NumTests:  2,
						NumFailed: 1,
						Duration:  0.15,
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.02,
								FailureOutput: &api.FailureOutput{
									Output: `=== RUN TestOne
--- FAIL: TestOne (0.02 seconds)
	file_test.go:11: Error message
	file_test.go:11: Longer
		error
		message.`,
								},
							},
							{
								Name:     "TestTwo",
								Duration: 0.13,
							},
						},
					},
				},
			},
		},
		{
			name:     "skip",
			testFile: "3.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:       "package/name",
						NumTests:   2,
						NumSkipped: 1,
						Duration:   0.15,
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.02,
								SkipMessage: &api.SkipMessage{
									Message: `=== RUN TestOne
--- SKIP: TestOne (0.02 seconds)
	file_test.go:11: Skip message`,
								},
							},
							{
								Name:     "TestTwo",
								Duration: 0.13,
							},
						},
					},
				},
			},
		},
		{
			name:     "go 1.4",
			testFile: "4.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:     "package/name",
						NumTests: 2,
						Duration: 0.16,
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.06,
							},
							{
								Name:     "TestTwo",
								Duration: 0.1,
							},
						},
					},
				},
			},
		},
		{
			name:     "multiple suites",
			testFile: "5.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:     "package/name1",
						NumTests: 2,
						Duration: 0.16,
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.06,
							},
							{
								Name:     "TestTwo",
								Duration: 0.1,
							},
						},
					},
					{
						Name:      "package/name2",
						NumTests:  2,
						Duration:  0.15,
						NumFailed: 1,
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.02,
								FailureOutput: &api.FailureOutput{
									Output: `=== RUN TestOne
--- FAIL: TestOne (0.02 seconds)
	file_test.go:11: Error message
	file_test.go:11: Longer
		error
		message.`,
								},
							},
							{
								Name:     "TestTwo",
								Duration: 0.13,
							},
						},
					},
				},
			},
		},
		{
			name:     "coverage statement",
			testFile: "6.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:     "package/name",
						NumTests: 2,
						Duration: 0.16,
						Properties: []*api.TestSuiteProperty{
							{
								Name:  "coverage.statements.pct",
								Value: "13.37",
							},
						},
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.06,
							},
							{
								Name:     "TestTwo",
								Duration: 0.1,
							},
						},
					},
				},
			},
		},
		{
			name:     "coverage statement in package result",
			testFile: "7.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:     "package/name",
						NumTests: 2,
						Duration: 0.16,
						Properties: []*api.TestSuiteProperty{
							{
								Name:  "coverage.statements.pct",
								Value: "10.0",
							},
						},
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.06,
							},
							{
								Name:     "TestTwo",
								Duration: 0.1,
							},
						},
					},
				},
			},
		},
		{
			name:     "go 1.5",
			testFile: "8.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:     "package/name",
						NumTests: 2,
						Duration: 0.05,
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.02,
							},
							{
								Name:     "TestTwo",
								Duration: 0.03,
							},
						},
					},
				},
			},
		},
		{
			name:     "nested",
			testFile: "9.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:     "package/name",
						NumTests: 2,
						Duration: 0.05,
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.02,
							},
							{
								Name:     "TestTwo",
								Duration: 0.03,
							},
						},
					},
					{
						Name:       "package/name/nested",
						NumTests:   2,
						NumFailed:  1,
						NumSkipped: 1,
						Duration:   0.05,
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.02,
								FailureOutput: &api.FailureOutput{
									Output: `=== RUN   TestOne
--- FAIL: TestOne (0.02 seconds)
	file_test.go:11: Error message
	file_test.go:11: Longer
		error
		message.`,
								},
							},
							{
								Name:     "TestTwo",
								Duration: 0.03,
								SkipMessage: &api.SkipMessage{
									Message: `=== RUN   TestTwo
--- SKIP: TestTwo (0.03 seconds)
	file_test.go:11: Skip message
PASS`,
								},
							},
						},
					},
					{
						Name:     "package/other/nested",
						NumTests: 2,
						Duration: 0.3,
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.1,
							},
							{
								Name:     "TestTwo",
								Duration: 0.2,
							},
						},
					},
				},
			},
		},
		{
			name:     "test case timing doesn't add to test suite timing",
			testFile: "10.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:     "package/name",
						NumTests: 2,
						Duration: 2.16,
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.06,
							},
							{
								Name:     "TestTwo",
								Duration: 0.1,
							},
						},
					},
				},
			},
		},
		{
			name:     "coverage statement in package result and inline",
			testFile: "11.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:     "package/name",
						NumTests: 2,
						Duration: 0.16,
						Properties: []*api.TestSuiteProperty{
							{
								Name:  "coverage.statements.pct",
								Value: "10.0",
							},
						},
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.06,
							},
							{
								Name:     "TestTwo",
								Duration: 0.1,
							},
						},
					},
				},
			},
		},
		{
			name:     "test case with subtests",
			testFile: "12.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:      "package/name",
						NumTests:  2,
						NumFailed: 1,
						Duration:  0.352,
						Properties: []*api.TestSuiteProperty{
							{
								Name:  "coverage.statements.pct",
								Value: "50.0",
							},
						},
						TestCases: []*api.TestCase{
							{
								Name:     "TestOne",
								Duration: 0.34,
								FailureOutput: &api.FailureOutput{
									Output: `=== RUN   TestOne
2017/03/27 14:33:13 top level log message
=== RUN   TestOne/sub1
2017/03/27 14:33:13 sub1 log message
=== RUN   TestOne/sub1/subsub1
2017/03/27 14:33:13 sub1 subsub1 failed
=== RUN   TestOne/sub1/subsub2
2017/03/27 14:33:14 sub1 subsub2 succeeded
=== RUN   TestOne/sub2
2017/03/27 14:33:14 sub2 succeeded
2017/03/27 14:33:14 another
multiline top level
log message
--- FAIL: TestOne (0.34s)
	foo_test.go:11: top level log message
    --- FAIL: TestOne/sub1 (0.31s)
    	foo_test.go:14: sub1 log message
        --- FAIL: TestOne/sub1/subsub1 (0.12s)
        	foo_test.go:18: sub1 subsub1 failed
        --- PASS: TestOne/sub1/subsub2 (0.15s)
        	foo_test.go:23: sub1 subsub2 succeeded
    --- PASS: TestOne/sub2 (0.01s)
    	foo_test.go:30: sub2 succeeded
	foo_test.go:34: another
		multiline top level
		log message`,
								},
							},
							{
								Name:     "TestTwo",
								Duration: 0,
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		parser := NewParser(flat.NewTestSuitesBuilder(), false)

		testFile := "./../../../test/gotest/testdata/" + testCase.testFile

		reader, err := os.Open(testFile)
		if err != nil {
			t.Errorf("%s: unexpected error opening file %q: %v", testCase.name, testFile, err)
			continue
		}
		testSuites, err := parser.Parse(bufio.NewScanner(reader))
		if err != nil {
			t.Errorf("%s: unexpected error parsing file: %v", testCase.name, err)
			continue
		}

		if !reflect.DeepEqual(testSuites, testCase.expectedSuites) {
			t.Errorf("%s: did not produce the correct test suites from file:\n\texpected:\n\t%v,\n\tgot\n\t%v", testCase.name, testCase.expectedSuites, testSuites)
		}
	}
}
