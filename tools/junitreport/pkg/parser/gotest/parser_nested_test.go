package gotest

import (
	"bufio"
	"os"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/diff"

	"github.com/openshift/origin/tools/junitreport/pkg/api"
	"github.com/openshift/origin/tools/junitreport/pkg/builder/nested"
)

// TestNestedParse tests that parsing the `go test` output in the test directory with a nested builder works as expected
func TestNestedParse(t *testing.T) {
	var testCases = []struct {
		name           string
		testFile       string
		rootSuiteNames []string
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
			name:           "basic with restricted root",
			testFile:       "1.txt",
			rootSuiteNames: []string{"package/name"},
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
									Output: "file_test.go:11: Error message\nfile_test.go:11: Longer\nerror\nmessage.\n",
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
									Message: "file_test.go:11: Skip message\n",
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
									Output: "file_test.go:11: Error message\nfile_test.go:11: Longer\nerror\nmessage.\n",
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
						Name:       "package/name",
						NumTests:   2,
						NumFailed:  0,
						NumSkipped: 0,
						Duration:   0.05,
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
									Output: "file_test.go:11: Error message\nfile_test.go:11: Longer\nerror\nmessage.\n",
								},
							},
							{
								Name:     "TestTwo",
								Duration: 0.03,
								SkipMessage: &api.SkipMessage{
									Message: "file_test.go:11: Skip message\n",
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
			name:     "nested tests with inline output",
			testFile: "14.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:      "parser/gotest",
						NumTests:  4,
						NumFailed: 2,
						Duration:  0.019,
						TestCases: []*api.TestCase{
							{
								Name: "TestSubTestWithFailures",
								FailureOutput: &api.FailureOutput{
									Output: "",
								},
							},
							{
								Name: "TestSubTestWithFailures/subtest-pass-1",
							},
							{
								Name: "TestSubTestWithFailures/subtest-pass-2",
							},
							{
								Name:      "TestSubTestWithFailures/subtest-fail-1",
								SystemOut: "text line\n",
								FailureOutput: &api.FailureOutput{
									Output: "data_parser_test.go:14: log line\ndata_parser_test.go:14: failed\n",
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "multi-suite nested output with coverage",
			testFile: "15.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:      "parser/gotest",
						NumTests:  4,
						NumFailed: 2,
						Duration:  0.019,
						TestCases: []*api.TestCase{
							{
								Name: "TestSubTestWithFailures",
								FailureOutput: &api.FailureOutput{
									Output: "",
								},
							},
							{
								Name: "TestSubTestWithFailures/subtest-pass-1",
							},
							{
								Name: "TestSubTestWithFailures/subtest-pass-2",
							},
							{
								Name:      "TestSubTestWithFailures/subtest-fail-1",
								SystemOut: "text line\n",
								FailureOutput: &api.FailureOutput{
									Output: "data_parser_test.go:14: log line\ndata_parser_test.go:14: failed\n",
								},
							},
						},
					},
					{
						Name:       "github.com/openshift/origin/tools/junitreport/pkg/parser/gotest/example",
						NumTests:   19,
						NumFailed:  9,
						Duration:   0.006,
						Properties: []*api.TestSuiteProperty{{Name: "coverage.statements.pct", Value: "0.0"}},
						TestCases: []*api.TestCase{
							{
								Name:          "TestSubTestWithFailures",
								FailureOutput: &api.FailureOutput{},
							},
							{Name: "TestSubTestWithFailures/subtest-pass-1"},
							{Name: "TestSubTestWithFailures/subtest-pass-2"},
							{
								Name:      "TestSubTestWithFailures/subtest-fail-1",
								SystemOut: "text line\n",
								FailureOutput: &api.FailureOutput{
									Output: "example_test.go:11: log line\nexample_test.go:11: failed\n",
								},
							},
							{
								Name:          "TestSubTestWithFirstFailures",
								FailureOutput: &api.FailureOutput{},
							},
							{
								Name:          "TestSubTestWithFirstFailures/subtest-fail-1",
								FailureOutput: &api.FailureOutput{Output: "example_test.go:15: log line\nexample_test.go:15: failed\n"},
								SystemOut:     "text line\n",
							},
							{Name: "TestSubTestWithFirstFailures/subtest-pass-1"},
							{Name: "TestSubTestWithFirstFailures/subtest-pass-2"},
							{
								Name:          "TestSubTestWithSubTestFailures",
								FailureOutput: &api.FailureOutput{},
							},
							{Name: "TestSubTestWithSubTestFailures/subtest-pass-1"},
							{Name: "TestSubTestWithSubTestFailures/subtest-pass-2"},
							{
								Name:          "TestSubTestWithSubTestFailures/subtest-fail-1",
								FailureOutput: &api.FailureOutput{Output: "example_test.go:25: log line before\nexample_test.go:29: log line after\n"},
								SystemOut:     "text line\n",
							},
							{Name: "TestSubTestWithSubTestFailures/subtest-fail-1/sub-subtest-pass-1"},
							{Name: "TestSubTestWithSubTestFailures/subtest-fail-1/sub-subtest-pass-2"},
							{
								Name:          "TestSubTestWithSubTestFailures/subtest-fail-1/sub-subtest-fail-1",
								FailureOutput: &api.FailureOutput{Output: "example_test.go:28: log line\nexample_test.go:28: failed\n"},
								SystemOut:     "text line\n",
							},
							{
								Name:          "TestSubTestWithMiddleFailures",
								FailureOutput: &api.FailureOutput{},
							},
							{Name: "TestSubTestWithMiddleFailures/subtest-pass-1"},
							{
								Name:          "TestSubTestWithMiddleFailures/subtest-fail-1",
								FailureOutput: &api.FailureOutput{Output: "example_test.go:35: log line\nexample_test.go:35: failed\n"},
								SystemOut:     "text line\n",
							},
							{Name: "TestSubTestWithMiddleFailures/subtest-pass-2"},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			parser := NewParser(nested.NewTestSuitesBuilder(testCase.rootSuiteNames), false)

			testFile := "./../../../test/gotest/testdata/" + testCase.testFile

			reader, err := os.Open(testFile)
			if err != nil {
				t.Fatalf("unexpected error opening file %q: %v", testFile, err)
			}
			testSuites, err := parser.Parse(bufio.NewScanner(reader))
			if err != nil {
				t.Fatalf("unexpected error parsing file: %v", err)
			}

			if !reflect.DeepEqual(testSuites, testCase.expectedSuites) {
				t.Errorf("did not produce the correct test suites from file:\n %s", diff.ObjectReflectDiff(testCase.expectedSuites, testSuites))
			}
		})
	}
}
