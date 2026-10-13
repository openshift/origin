package oscmd

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
						Duration: 11.245,
						Children: []*api.TestSuite{
							{
								Name:     "package/name",
								NumTests: 2,
								Duration: 11.245,
								TestCases: []*api.TestCase{
									{
										Name:     `package/name/file.sh:23: executing 'some command' expecting success`,
										Duration: 0.123,
									},
									{
										Name:     `package/name/file.sh:24: executing 'some other command' expecting success`,
										Duration: 11.123,
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
						Duration: 11.245,
						TestCases: []*api.TestCase{
							{
								Name:     `package/name/file.sh:23: executing 'some command' expecting success`,
								Duration: 0.123,
							},
							{
								Name:     `package/name/file.sh:24: executing 'some other command' expecting success`,
								Duration: 11.123,
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
						Duration:  11.245,
						Children: []*api.TestSuite{
							{
								Name:      "package/name",
								NumTests:  2,
								NumFailed: 1,
								Duration:  11.245,
								TestCases: []*api.TestCase{
									{
										Name:     `package/name/file.sh:23: executing 'some command' expecting success`,
										Duration: 0.123,
										FailureOutput: &api.FailureOutput{
											Output: `=== BEGIN TEST CASE ===
package/name/file.sh:23: executing 'some command' expecting success
FAILURE after 0.1234s: package/name/file.sh:23: executing 'some command' expecting success: the command returned the wrong error code
There was no output from the command.
There was no error output from the command.
=== END TEST CASE ===`,
											Message: "the command returned the wrong error code",
										},
									},
									{
										Name:     `package/name/file.sh:24: executing 'some other command' expecting success`,
										Duration: 11.123,
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
			testFile: "3.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:      "package",
						NumTests:  4,
						Duration:  22.49,
						NumFailed: 1,
						Children: []*api.TestSuite{
							{
								Name:     "package/name",
								NumTests: 2,
								Duration: 11.245,
								TestCases: []*api.TestCase{
									{
										Name:     `package/name/file.sh:23: executing 'some command' expecting success`,
										Duration: 0.123,
									},
									{
										Name:     `package/name/file.sh:24: executing 'some other command' expecting success`,
										Duration: 11.123,
									},
								},
							},
							{
								Name:      "package/name2",
								NumTests:  2,
								NumFailed: 1,
								Duration:  11.245,
								TestCases: []*api.TestCase{
									{
										Name:     `package/name/file.sh:23: executing 'some command' expecting success`,
										Duration: 0.123,
										FailureOutput: &api.FailureOutput{
											Output: `=== BEGIN TEST CASE ===
package/name/file.sh:23: executing 'some command' expecting success
FAILURE after 0.1234s: package/name/file.sh:23: executing 'some command' expecting success: the command returned the wrong error code
There was no output from the command.
There was no error output from the command.
=== END TEST CASE ===`,
											Message: "the command returned the wrong error code",
										},
									},
									{
										Name:     `package/name/file.sh:24: executing 'some other command' expecting success`,
										Duration: 11.123,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "nested",
			testFile: "4.txt",
			expectedSuites: &api.TestSuites{
				Suites: []*api.TestSuite{
					{
						Name:      "package",
						NumTests:  6,
						Duration:  33.735,
						NumFailed: 1,
						Children: []*api.TestSuite{
							{
								Name:      "package/name",
								NumTests:  4,
								Duration:  22.49,
								NumFailed: 1,
								TestCases: []*api.TestCase{
									{
										Name:     `package/name/file.sh:23: executing 'some command' expecting success`,
										Duration: 0.123,
									},
									{
										Name:     `package/name/file.sh:24: executing 'some other command' expecting success`,
										Duration: 11.123,
									},
								},
								Children: []*api.TestSuite{
									{
										Name:      "package/name/nested",
										NumTests:  2,
										NumFailed: 1,
										Duration:  11.245,
										TestCases: []*api.TestCase{
											{
												Name:     `package/name/file.sh:23: executing 'some command' expecting success`,
												Duration: 0.123,
												FailureOutput: &api.FailureOutput{
													Output: `=== BEGIN TEST CASE ===
package/name/file.sh:23: executing 'some command' expecting success
FAILURE after 0.1234s: package/name/file.sh:23: executing 'some command' expecting success: the command returned the wrong error code
There was no output from the command.
There was no error output from the command.
=== END TEST CASE ===`,
													Message: "the command returned the wrong error code",
												},
											},
											{
												Name:     `package/name/file.sh:24: executing 'some other command' expecting success`,
												Duration: 11.123,
											},
										},
									},
								},
							},
							{
								Name:     "package/other",
								NumTests: 2,
								Duration: 11.245,
								Children: []*api.TestSuite{
									{
										Name:     "package/other/nested",
										NumTests: 2,
										Duration: 11.245,
										TestCases: []*api.TestCase{
											{
												Name:     `package/name/file.sh:23: executing 'some command' expecting success`,
												Duration: 0.123,
											},
											{
												Name:     `package/name/file.sh:24: executing 'some other command' expecting success`,
												Duration: 11.123,
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
	}

	for _, testCase := range testCases {
		parser := NewParser(nested.NewTestSuitesBuilder(testCase.rootSuiteNames), false)

		testFile := "./../../../test/oscmd/testdata/" + testCase.testFile

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
