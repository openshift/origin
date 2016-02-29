package gotest

import (
	"bufio"
	"os"
	"reflect"
	"testing"

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
						Name:     "package",
						NumTests: 2,
						Duration: 0.16,
						Children: []*api.TestSuite{
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
						Name:      "package",
						NumTests:  2,
						NumFailed: 1,
						Duration:  0.15,
						Children: []*api.TestSuite{
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
			},
		},
		{
			name:     "skip",
			testFile: "3.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:       "package",
						NumTests:   2,
						NumSkipped: 1,
						Duration:   0.15,
						Children: []*api.TestSuite{
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
			},
		},
		{
			name:     "go 1.4",
			testFile: "4.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:     "package",
						NumTests: 2,
						Duration: 0.16,
						Children: []*api.TestSuite{
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
			},
		},
		{
			name:     "multiple suites",
			testFile: "5.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:      "package",
						NumTests:  4,
						NumFailed: 1,
						Duration:  0.31,
						Children: []*api.TestSuite{
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
			},
		},
		{
			name:     "coverage statement",
			testFile: "6.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:     "package",
						NumTests: 2,
						Duration: 0.16,
						Children: []*api.TestSuite{
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
			},
		},
		{
			name:     "coverage statement in package result",
			testFile: "7.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:     "package",
						NumTests: 2,
						Duration: 0.16,
						Children: []*api.TestSuite{
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
			},
		},
		{
			name:     "go 1.5",
			testFile: "8.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:     "package",
						NumTests: 2,
						Duration: 0.05,
						Children: []*api.TestSuite{
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
			},
		},
		{
			name:     "nested ",
			testFile: "9.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:       "package",
						NumTests:   6,
						NumFailed:  1,
						NumSkipped: 1,
						Duration:   0.4,
						Children: []*api.TestSuite{
							{
								Name:       "package/name",
								NumTests:   4,
								NumFailed:  1,
								NumSkipped: 1,
								Duration:   0.1,
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
								Children: []*api.TestSuite{

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
PASS`, // we include this line greedily even though it does not belong to the test
												},
											},
										},
									},
								},
							},
							{
								Name:     "package/other",
								NumTests: 2,
								Duration: 0.3,
								Children: []*api.TestSuite{

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
						Name:     "package",
						NumTests: 2,
						Duration: 2.16,
						Children: []*api.TestSuite{
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
			},
		},
		{
			name:     "coverage statement in package result and inline",
			testFile: "11.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:     "package",
						NumTests: 2,
						Duration: 0.16,
						Children: []*api.TestSuite{
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
			},
		},
	}

	for _, testCase := range testCases {
		parser := NewParser(nested.NewTestSuitesBuilder(testCase.rootSuiteNames), false)

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
