package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/jstemmer/go-junit-report/parser"
)

type TestCase struct {
	name        string
	reportName  string
	report      *parser.Report
	noXMLHeader bool
	packageName string
}

var testCases = []TestCase{
	{
		name:       "01-pass.txt",
		reportName: "01-report.xml",
		report: &parser.Report{
			Packages: []*parser.Package{
				{
					Name: "package",
					Children: []*parser.Package{
						{
							Name: "package/name",
							Time: 160,
							Tests: []*parser.Test{
								{
									Name:   "TestZ",
									Time:   60,
									Result: parser.PASS,
									Output: []string{},
								},
								{
									Name:   "TestA",
									Time:   100,
									Result: parser.PASS,
									Output: []string{},
								},
							},
						},
					},
				},
			},
		},
	},
	{
		name:       "02-fail.txt",
		reportName: "02-report.xml",
		report: &parser.Report{
			Packages: []*parser.Package{
				{
					Name: "package",
					Children: []*parser.Package{
						{
							Name: "package/name",
							Time: 151,
							Tests: []*parser.Test{
								{
									Name:   "TestOne",
									Time:   20,
									Result: parser.FAIL,
									Output: []string{
										"file_test.go:11: Error message",
										"file_test.go:11: Longer",
										"\terror",
										"\tmessage.",
									},
								},
								{
									Name:   "TestTwo",
									Time:   130,
									Result: parser.PASS,
									Output: []string{},
								},
							},
						},
					},
				},
			},
		},
	},
	{
		name:       "03-skip.txt",
		reportName: "03-report.xml",
		report: &parser.Report{
			Packages: []*parser.Package{
				{
					Name: "package",
					Children: []*parser.Package{
						{
							Name: "package/name",
							Time: 150,
							Tests: []*parser.Test{
								{
									Name:   "TestOne",
									Time:   20,
									Result: parser.SKIP,
									Output: []string{
										"file_test.go:11: Skip message",
									},
								},
								{
									Name:   "TestTwo",
									Time:   130,
									Result: parser.PASS,
									Output: []string{},
								},
							},
						},
					},
				},
			},
		},
	},
	{
		name:       "04-go_1_4.txt",
		reportName: "04-report.xml",
		report: &parser.Report{
			Packages: []*parser.Package{
				{
					Name: "package",
					Children: []*parser.Package{
						{
							Name: "package/name",
							Time: 160,
							Tests: []*parser.Test{
								{
									Name:   "TestOne",
									Time:   60,
									Result: parser.PASS,
									Output: []string{},
								},
								{
									Name:   "TestTwo",
									Time:   100,
									Result: parser.PASS,
									Output: []string{},
								},
							},
						},
					},
				},
			},
		},
	},
	{
		name:       "05-no_xml_header.txt",
		reportName: "05-report.xml",
		report: &parser.Report{
			Packages: []*parser.Package{
				{
					Name: "package",
					Children: []*parser.Package{
						{
							Name: "package/name",
							Time: 160,
							Tests: []*parser.Test{
								{
									Name:   "TestOne",
									Time:   60,
									Result: parser.PASS,
									Output: []string{},
								},
								{
									Name:   "TestTwo",
									Time:   100,
									Result: parser.PASS,
									Output: []string{},
								},
							},
						},
					},
				},
			},
		},
		noXMLHeader: true,
	},
	{
		name:       "06-mixed.txt",
		reportName: "06-report.xml",
		report: &parser.Report{
			Packages: []*parser.Package{
				{
					Name: "package",
					Children: []*parser.Package{
						{
							Name: "package/name1",
							Time: 160,
							Tests: []*parser.Test{
								{
									Name:   "TestOne",
									Time:   60,
									Result: parser.PASS,
									Output: []string{},
								},
								{
									Name:   "TestTwo",
									Time:   100,
									Result: parser.PASS,
									Output: []string{},
								},
							},
						},
						{
							Name: "package/name2",
							Time: 151,
							Tests: []*parser.Test{
								{
									Name:   "TestOne",
									Time:   20,
									Result: parser.FAIL,
									Output: []string{
										"file_test.go:11: Error message",
										"file_test.go:11: Longer",
										"\terror",
										"\tmessage.",
									},
								},
								{
									Name:   "TestTwo",
									Time:   130,
									Result: parser.PASS,
									Output: []string{},
								},
							},
						},
					},
				},
			},
		},
		noXMLHeader: true,
	},
	{
		name:       "07-compiled_test.txt",
		reportName: "07-report.xml",
		report: &parser.Report{
			Packages: []*parser.Package{
				{

					Name: "test/package",
					Time: 160,
					Tests: []*parser.Test{
						{
							Name:   "TestOne",
							Time:   60,
							Result: parser.PASS,
							Output: []string{},
						},
						{
							Name:   "TestTwo",
							Time:   100,
							Result: parser.PASS,
							Output: []string{},
						},
					},
				},
			},
		},
		packageName: "test/package",
	},
	{
		name:       "08-parallel.txt",
		reportName: "08-report.xml",
		report: &parser.Report{
			Packages: []*parser.Package{
				{
					Name: "github.com",
					Children: []*parser.Package{
						{
							Name: "github.com/dmitris",
							Children: []*parser.Package{
								{
									Name: "github.com/dmitris/test-go-junit-report",
									Time: 440,
									Tests: []*parser.Test{
										{
											Name:   "TestDoFoo",
											Time:   270,
											Result: parser.PASS,
											Output: []string{"cov_test.go:10: DoFoo log 1", "cov_test.go:10: DoFoo log 2"},
										},
										{
											Name:   "TestDoFoo2",
											Time:   160,
											Result: parser.PASS,
											Output: []string{"cov_test.go:21: DoFoo2 log 1", "cov_test.go:21: DoFoo2 log 2"},
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
		name:       "09-coverage.txt",
		reportName: "09-report.xml",
		report: &parser.Report{
			Packages: []*parser.Package{
				{
					Name: "package",
					Children: []*parser.Package{
						{
							Name: "package/name",
							Time: 160,
							Tests: []*parser.Test{
								{
									Name:   "TestZ",
									Time:   60,
									Result: parser.PASS,
									Output: []string{},
								},
								{
									Name:   "TestA",
									Time:   100,
									Result: parser.PASS,
									Output: []string{},
								},
							},
							CoveragePct: "13.37",
						},
					},
				},
			},
		},
	},
	{
		name:       "10-multipkg-coverage.txt",
		reportName: "10-report.xml",
		report: &parser.Report{
			Packages: []*parser.Package{
				{
					Name: "package1",
					Children: []*parser.Package{
						{
							Name: "package1/foo",
							Time: 400,
							Tests: []*parser.Test{
								{
									Name:   "TestA",
									Time:   100,
									Result: parser.PASS,
									Output: []string{},
								},
								{
									Name:   "TestB",
									Time:   300,
									Result: parser.PASS,
									Output: []string{},
								},
							},
							CoveragePct: "10.0",
						},
					},
				},
				{
					Name: "package2",
					Children: []*parser.Package{
						{
							Name: "package2/bar",
							Time: 4200,
							Tests: []*parser.Test{
								{
									Name:   "TestC",
									Time:   4200,
									Result: parser.PASS,
									Output: []string{},
								},
							},
							CoveragePct: "99.8",
						},
					},
				},
			},
		},
	},
	{
		name:       "11-go_1_5.txt",
		reportName: "11-report.xml",
		report: &parser.Report{
			Packages: []*parser.Package{
				{
					Name: "package",
					Children: []*parser.Package{
						{
							Name: "package/name",
							Time: 50,
							Tests: []*parser.Test{
								{
									Name:   "TestOne",
									Time:   20,
									Result: parser.PASS,
									Output: []string{},
								},
								{
									Name:   "TestTwo",
									Time:   30,
									Result: parser.PASS,
									Output: []string{},
								},
							},
						},
					},
				},
			},
		},
	},
	{
		name:       "12-nested.txt",
		reportName: "12-report.xml",
		report: &parser.Report{
			Packages: []*parser.Package{
				{
					Name: "package1",
					Time: 0,
					Children: []*parser.Package{
						{
							Name: "package1/foo",
							Time: 400,
							Tests: []*parser.Test{
								{
									Name:   "TestA",
									Time:   100,
									Result: parser.PASS,
									Output: []string{},
								},
								{
									Name:   "TestB",
									Time:   300,
									Result: parser.PASS,
									Output: []string{},
								},
							},
							CoveragePct: "10.0",
						},
						{
							Name: "package1/bar",
							Time: 4200,
							Tests: []*parser.Test{
								{
									Name:   "TestC",
									Time:   4200,
									Result: parser.PASS,
									Output: []string{},
								},
							},
							CoveragePct: "99.8",
							Children: []*parser.Package{
								{
									Name: "package1/bar/baz",
									Time: 8400,
									Tests: []*parser.Test{
										{
											Name:   "TestD",
											Time:   8400,
											Result: parser.PASS,
											Output: []string{},
										},
									},
									CoveragePct: "20.0",
								},
							},
						},
					},
				},
			},
		},
	},
}

func TestParser(t *testing.T) {
	for _, testCase := range testCases {
		t.Logf("Running: %s", testCase.name)

		file, err := os.Open("tests/" + testCase.name)
		if err != nil {
			t.Fatal(err)
		}

		report, err := parser.Parse(file, testCase.packageName)
		if err != nil {
			t.Fatalf("error parsing: %s", err)
		}

		if report == nil {
			t.Fatalf("Report == nil")
		}

		expected := testCase.report
		if len(report.Packages) != len(expected.Packages) {
			fmt.Printf("%s", report)
			t.Fatalf("Report packages == %d, want %d", len(report.Packages), len(expected.Packages))
		}

		for i, pkg := range report.Packages {
			expPkg := expected.Packages[i]

			checkPackages(expPkg, pkg, t)
		}
	}
}

func checkPackages(expPkg, pkg *parser.Package, t *testing.T) {
	if pkg.Name != expPkg.Name {
		t.Errorf("Package.Name == %s, want %s", pkg.Name, expPkg.Name)
	}

	if pkg.Time != expPkg.Time {
		t.Errorf("Package.Time == %d, want %d", pkg.Time, expPkg.Time)
	}

	if len(pkg.Tests) != len(expPkg.Tests) {
		t.Fatalf("Package Tests == %d, want %d", len(pkg.Tests), len(expPkg.Tests))
	}

	if len(pkg.Children) != len(expPkg.Children) {
		t.Fatalf("Package Children == %d, want %d", len(pkg.Children), len(expPkg.Children))
	}

	for j, test := range pkg.Tests {
		expTest := expPkg.Tests[j]

		if test.Name != expTest.Name {
			t.Errorf("Test.Name == %s, want %s", test.Name, expTest.Name)
		}

		if test.Time != expTest.Time {
			t.Errorf("Test.Time == %d, want %d", test.Time, expTest.Time)
		}

		if test.Result != expTest.Result {
			t.Errorf("Test.Result == %d, want %d", test.Result, expTest.Result)
		}

		testOutput := strings.Join(test.Output, "\n")
		expTestOutput := strings.Join(expTest.Output, "\n")
		if testOutput != expTestOutput {
			t.Errorf("Test.Output ==\n%s\n, want\n%s", testOutput, expTestOutput)
		}
	}
	if pkg.CoveragePct != expPkg.CoveragePct {
		t.Errorf("Package.CoveragePct == %s, want %s", pkg.CoveragePct, expPkg.CoveragePct)
	}

	for i, child := range pkg.Children {
		expChild := expPkg.Children[i]

		checkPackages(expChild, child, t)
	}
}

func TestJUnitFormatter(t *testing.T) {
	for _, testCase := range testCases {
		report, err := loadTestReport(testCase.reportName)
		if err != nil {
			t.Fatal(err)
		}

		var junitReport bytes.Buffer

		if err = JUnitReportXML(testCase.report, testCase.noXMLHeader, &junitReport); err != nil {
			t.Fatal(err)
		}

		if string(junitReport.Bytes()) != report {
			t.Fatalf("Fail: %s Report xml ==\n%s, want\n%s", testCase.name, string(junitReport.Bytes()), report)
		}
	}
}

func loadTestReport(name string) (string, error) {
	contents, err := ioutil.ReadFile("tests/" + name)
	if err != nil {
		return "", err
	}

	// replace value="1.0" With actual version
	report := strings.Replace(string(contents), `value="1.0"`, fmt.Sprintf(`value="%s"`, runtime.Version()), -1)

	return report, nil
}
