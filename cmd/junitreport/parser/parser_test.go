package parser

import (
	"os"
	"reflect"
	"testing"

	upstream "github.com/jstemmer/go-junit-report/parser"
)

func TestParseFile(t *testing.T) {
	var testCases = []struct {
		name           string
		file           string
		expectedReport *upstream.Report
		expectedError  error
	}{
		{
			name: "package with no tests",
			file: "1-package_no_tests.txt",
			expectedReport: &upstream.Report{
				Packages: []*upstream.Package{
					{
						Name: "/test/package/title",
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "package with one test",
			file: "2-package_with_test.txt",
			expectedReport: &upstream.Report{
				Packages: []*upstream.Package{
					{
						Name: "/test/package/title",
						Time: 0,
						Tests: []*upstream.Test{
							{
								Name:   "/test/package/test.sh:10: func 'arg' 'arg'",
								Time:   3,
								Result: upstream.PASS,
							},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "package with one test and test output",
			file: "3-package_with_test_output.txt",
			expectedReport: &upstream.Report{
				Packages: []*upstream.Package{
					{
						Name: "/test/package/title",
						Time: 0,
						Tests: []*upstream.Test{
							{
								Name:   "/test/package/test.sh:10: func 'arg' 'arg'",
								Time:   3,
								Result: upstream.PASS,
								Output: []string{
									"output line 1",
									"output line 2",
									"output line 3",
								},
							},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "package with tests",
			file: "4-package_with_tests.txt",
			expectedReport: &upstream.Report{
				Packages: []*upstream.Package{
					{
						Name: "/test/package/title",
						Time: 0,
						Tests: []*upstream.Test{
							{
								Name:   "/test/package/test.sh:10: func 'arg' 'arg'",
								Time:   3,
								Result: upstream.PASS,
								Output: []string{
									"output line 1",
									"output line 2",
									"output line 3",
								},
							},
							{
								Name:   "/test/package/test.sh:15: other_func 'arg' 'other_arg'",
								Time:   0,
								Result: upstream.FAIL,
								Output: []string{
									"output line 0",
									"output line 1",
									"output line 2",
								},
							},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "packages with no tests",
			file: "5-packages.txt",
			expectedReport: &upstream.Report{
				Packages: []*upstream.Package{
					{
						Name: "/test/package/title",
					},
					{
						Name: "/test/package/other",
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "nested packages",
			file: "6-nested_packages.txt",
			expectedReport: &upstream.Report{
				Packages: []*upstream.Package{
					{
						Name: "/test/package/title",
						Children: []*upstream.Package{
							{
								Name: "/test/package/other",
							},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "many nested packages",
			file: "7-many_nested_packages.txt",
			expectedReport: &upstream.Report{
				Packages: []*upstream.Package{
					{
						Name: "/test/package/title",
						Children: []*upstream.Package{
							{
								Name: "/test/package/other",
							},
						},
					},
					{
						Name: "/test/package/third",
						Children: []*upstream.Package{
							{
								Name: "/test/package/different",
							},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "many nested packages with tests",
			file: "8-many_nested_packages_with_tests.txt",
			expectedReport: &upstream.Report{
				Packages: []*upstream.Package{
					{
						Name: "/test/package/title",
						Tests: []*upstream.Test{
							{
								Name:   "/test/package/test.sh:15: other_func 'arg' 'other_arg'",
								Time:   0,
								Result: upstream.FAIL,
								Output: []string{
									"output line 0",
									"output line 1",
									"output line 2",
								},
							},
						},
						Children: []*upstream.Package{
							{
								Name: "/test/package/other",
								Tests: []*upstream.Test{
									{
										Name:   "/test/package/test.sh:10: func 'arg' 'arg'",
										Time:   3,
										Result: upstream.PASS,
										Output: []string{
											"output line 1",
											"output line 2",
											"output line 3",
										},
									},
								},
							},
						},
					},
					{
						Name: "/test/package/third",
						Tests: []*upstream.Test{
							{
								Name:   "/test/package/test.sh:20: third_func 'arg' 'diff_arg'",
								Time:   13,
								Result: upstream.SKIP,
								Output: []string{
									"output line 3",
									"output line 2",
									"output line 1",
								},
							},
							{
								Name:   "/test/package/test.sh:35: other_func 'arg' 'diff_arg'",
								Time:   21,
								Result: upstream.FAIL,
								Output: []string{
									"output line 2",
									"output line 1",
									"output line 0",
								},
							},
						},
						Children: []*upstream.Package{
							{
								Name: "/test/package/different",
								Tests: []*upstream.Test{
									{
										Name:   "/test/package/test.sh:25: diff_func 'arg' 'other_arg'",
										Time:   20,
										Result: upstream.PASS,
										Output: []string{
											"output line 0",
											"output line 2",
											"output line 1",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedError: nil,
		},
	}

	for _, testCase := range testCases {
		file, err := os.Open("test/" + testCase.file)
		if err != nil {
			t.Fatalf("unexpected error opening test file: %v", err)
		}

		report, err := Parse(file)
		if err != nil {
			t.Fatalf("error parsing: %s", err)
		}

		if !reflect.DeepEqual(report, testCase.expectedReport) {
			t.Errorf("%s: did not get correct report from file:\n\texpected:\n\t%s\n\tgot:\n\t%s\n", testCase.name, testCase.expectedReport, report)
		}
		if !reflect.DeepEqual(err, testCase.expectedError) {
			t.Errorf("%s: did not get correct error:\n\texpected:\n\t%s\n\tgot:\n\t%s\n", testCase.name, testCase.expectedError, err)
		}
	}
}
