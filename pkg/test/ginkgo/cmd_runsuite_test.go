package ginkgo

import (
	"strings"
	"testing"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/openshift/origin/pkg/test/extensions"
)

func Test_detectDuplicateTests(t *testing.T) {
	tests := []struct {
		name                string
		specs               extensions.ExtensionTestSpecs
		wantTestCaseCount   int
		wantSuccess         bool
		wantFailure         bool
		wantDuplicateCount  int
		wantFailureContains []string
	}{
		{
			name: "no duplicates",
			specs: extensions.ExtensionTestSpecs{
				{
					ExtensionTestSpec: &extensiontests.ExtensionTestSpec{
						Name:   "test-1",
						Source: "source-1",
					},
				},
				{
					ExtensionTestSpec: &extensiontests.ExtensionTestSpec{
						Name:   "test-2",
						Source: "source-2",
					},
				},
			},
			wantTestCaseCount: 1,
			wantSuccess:       true,
			wantFailure:       false,
		},
		{
			name: "single duplicate test",
			specs: extensions.ExtensionTestSpecs{
				{
					ExtensionTestSpec: &extensiontests.ExtensionTestSpec{
						Name:          "duplicate-test",
						Source:        "source-1",
						CodeLocations: []string{"file1.go:10"},
					},
				},
				{
					ExtensionTestSpec: &extensiontests.ExtensionTestSpec{
						Name:          "duplicate-test",
						Source:        "source-2",
						CodeLocations: []string{"file2.go:20"},
					},
				},
			},
			wantTestCaseCount:  2,
			wantSuccess:        true,
			wantFailure:        true,
			wantDuplicateCount: 1,
			wantFailureContains: []string{
				"Found 1 duplicate tests",
				"duplicate-test",
				"=== Duplicate Test: duplicate-test ===",
				"Found 2 occurrences:",
				"Occurrence 1:",
				"Source: source-1",
				"CodeLocations: file1.go:10",
				"Occurrence 2:",
				"Source: source-2",
				"CodeLocations: file2.go:20",
			},
		},
		{
			name: "multiple duplicate tests",
			specs: extensions.ExtensionTestSpecs{
				{
					ExtensionTestSpec: &extensiontests.ExtensionTestSpec{
						Name:          "test-a",
						Source:        "source-a1",
						CodeLocations: []string{"file-a1.go:100"},
					},
				},
				{
					ExtensionTestSpec: &extensiontests.ExtensionTestSpec{
						Name:          "test-a",
						Source:        "source-a2",
						CodeLocations: []string{"file-a2.go:200"},
					},
				},
				{
					ExtensionTestSpec: &extensiontests.ExtensionTestSpec{
						Name:          "test-b",
						Source:        "source-b1",
						CodeLocations: []string{"file-b1.go:300"},
					},
				},
				{
					ExtensionTestSpec: &extensiontests.ExtensionTestSpec{
						Name:          "test-b",
						Source:        "source-b2",
						CodeLocations: []string{"file-b2.go:400"},
					},
				},
				{
					ExtensionTestSpec: &extensiontests.ExtensionTestSpec{
						Name:          "test-b",
						Source:        "source-b3",
						CodeLocations: []string{"file-b3.go:500"},
					},
				},
				{
					ExtensionTestSpec: &extensiontests.ExtensionTestSpec{
						Name:   "unique-test",
						Source: "source-unique",
					},
				},
			},
			wantTestCaseCount:  2,
			wantSuccess:        true,
			wantFailure:        true,
			wantDuplicateCount: 2,
			wantFailureContains: []string{
				"Found 2 duplicate tests",
				"test-a",
				"test-b",
				"=== Duplicate Test: test-a ===",
				"Found 2 occurrences:",
				"=== Duplicate Test: test-b ===",
				"Found 3 occurrences:",
				"source-a1",
				"source-a2",
				"source-b1",
				"source-b2",
				"source-b3",
				"file-a1.go:100",
				"file-a2.go:200",
				"file-b1.go:300",
				"file-b2.go:400",
				"file-b3.go:500",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCases := detectDuplicateTests(tt.specs)

			// Check test case count
			if len(testCases) != tt.wantTestCaseCount {
				t.Errorf("detectDuplicateTests() returned %d test cases, want %d", len(testCases), tt.wantTestCaseCount)
			}

			// Check for success test case
			hasSuccess := false
			hasFailure := false
			var failureOutput string

			for _, tc := range testCases {
				if tc.Name != "There should not be duplicate tests" {
					t.Errorf("test case has wrong name: %q", tc.Name)
				}

				if tc.FailureOutput == nil {
					hasSuccess = true
				} else {
					hasFailure = true
					failureOutput = tc.FailureOutput.Output
				}
			}

			if hasSuccess != tt.wantSuccess {
				t.Errorf("detectDuplicateTests() hasSuccess = %v, want %v", hasSuccess, tt.wantSuccess)
			}

			if hasFailure != tt.wantFailure {
				t.Errorf("detectDuplicateTests() hasFailure = %v, want %v", hasFailure, tt.wantFailure)
			}

			// If we expect failure, check the failure output content
			if tt.wantFailure && hasFailure {
				for _, want := range tt.wantFailureContains {
					if !strings.Contains(failureOutput, want) {
						t.Errorf("failure output missing expected string %q\nGot:\n%s", want, failureOutput)
					}
				}

				// Print the actual failure output for review
				t.Logf("Failure output:\n%s", failureOutput)
			}
		})
	}
}
