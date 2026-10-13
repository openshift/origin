package ginkgo

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/openshift-eng/openshift-tests-extension/pkg/dbtime"
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/openshift/origin/pkg/test/extensions"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

func Test_lastLines(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		max     int
		matches []string
		want    string
	}{
		{output: "", max: 0, want: ""},
		{output: "", max: 1, want: ""},
		{output: "test", max: 1, want: "test"},
		{output: "test\n", max: 1, want: "test"},
		{output: "test\nother", max: 1, want: "other"},
		{output: "test\nother\n", max: 1, want: "other"},
		{output: "test\nother\n", max: 2, want: "test\nother"},
		{output: "test\nother\n", max: 3, want: "test\nother"},
		{output: "test\n\n\nother\n", max: 2, want: "test\n\n\nother"},

		{output: "test\n\n\nother and stuff\n", max: 2, matches: []string{"other"}, want: "other and stuff"},
		{output: "test\n\n\nother\n", max: 2, matches: []string{"test"}, want: "test\n\n\nother"},
		{output: "test\n\n\nother\n", max: 1, matches: []string{"test"}, want: "other"},
		{output: "test\ntest\n\n\nother\n", max: 10, matches: []string{"test"}, want: "test\n\n\nother"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lastLinesUntil(tt.output, tt.max, tt.matches...); got != tt.want {
				t.Errorf("lastLines() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_populateOTEMetadata(t *testing.T) {
	startTime := time.Date(2023, 12, 25, 10, 0, 0, 0, time.UTC)
	endTime := time.Date(2023, 12, 25, 10, 5, 0, 0, time.UTC)

	tests := []struct {
		name                 string
		extensionResult      *extensions.ExtensionTestResult
		expectedLifecycle    string
		expectedStartTime    string
		expectedEndTime      string
		expectedSourceImage  string
		expectedSourceBinary string
		expectedSourceURL    string
		expectedSourceCommit string
	}{
		{
			name:                 "nil extension result",
			extensionResult:      nil,
			expectedLifecycle:    "",
			expectedStartTime:    "",
			expectedEndTime:      "",
			expectedSourceImage:  "",
			expectedSourceBinary: "",
			expectedSourceURL:    "",
			expectedSourceCommit: "",
		},
		{
			name: "complete extension result",
			extensionResult: &extensions.ExtensionTestResult{
				ExtensionTestResult: &extensiontests.ExtensionTestResult{
					Name:      "test-case",
					Lifecycle: extensiontests.LifecycleBlocking,
					StartTime: dbtime.Ptr(startTime),
					EndTime:   dbtime.Ptr(endTime),
				},
				Source: extensions.Source{
					Source: &extension.Source{
						Commit:    "abc123def456",
						SourceURL: "https://github.com/example/repo",
					},
					SourceImage:  "tests",
					SourceBinary: "openshift-tests",
				},
			},
			expectedLifecycle:    "blocking",
			expectedStartTime:    "2023-12-25T10:00:00Z",
			expectedEndTime:      "2023-12-25T10:05:00Z",
			expectedSourceImage:  "tests",
			expectedSourceBinary: "openshift-tests",
			expectedSourceURL:    "https://github.com/example/repo",
			expectedSourceCommit: "abc123def456",
		},
		{
			name: "informing lifecycle",
			extensionResult: &extensions.ExtensionTestResult{
				ExtensionTestResult: &extensiontests.ExtensionTestResult{
					Name:      "test-case",
					Lifecycle: extensiontests.LifecycleInforming,
					StartTime: dbtime.Ptr(startTime),
					EndTime:   dbtime.Ptr(endTime),
				},
				Source: extensions.Source{
					Source: &extension.Source{
						Commit:    "xyz789",
						SourceURL: "https://github.com/openshift/origin",
					},
					SourceImage:  "tests",
					SourceBinary: "openshift-tests",
				},
			},
			expectedLifecycle:    "informing",
			expectedStartTime:    "2023-12-25T10:00:00Z",
			expectedEndTime:      "2023-12-25T10:05:00Z",
			expectedSourceImage:  "tests",
			expectedSourceBinary: "openshift-tests",
			expectedSourceURL:    "https://github.com/openshift/origin",
			expectedSourceCommit: "xyz789",
		},
		{
			name: "missing time fields",
			extensionResult: &extensions.ExtensionTestResult{
				ExtensionTestResult: &extensiontests.ExtensionTestResult{
					Name:      "test-case",
					Lifecycle: extensiontests.LifecycleBlocking,
					StartTime: nil,
					EndTime:   nil,
				},
			},
			expectedLifecycle:    "blocking",
			expectedStartTime:    "",
			expectedEndTime:      "",
			expectedSourceImage:  "",
			expectedSourceBinary: "",
			expectedSourceURL:    "",
			expectedSourceCommit: "",
		},
		{
			name: "partial source information",
			extensionResult: &extensions.ExtensionTestResult{
				ExtensionTestResult: &extensiontests.ExtensionTestResult{
					Name:      "test-case",
					Lifecycle: extensiontests.LifecycleBlocking,
					StartTime: dbtime.Ptr(startTime),
					EndTime:   dbtime.Ptr(endTime),
				},
				Source: extensions.Source{
					Source: &extension.Source{
						Commit: "abc123",
					},
					SourceImage: "tests",
				},
			},
			expectedLifecycle:    "blocking",
			expectedStartTime:    "2023-12-25T10:00:00Z",
			expectedEndTime:      "2023-12-25T10:05:00Z",
			expectedSourceImage:  "tests",
			expectedSourceBinary: "",
			expectedSourceURL:    "",
			expectedSourceCommit: "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCase := &junitapi.JUnitTestCase{
				Name:     "test-case",
				Duration: 300.0, // 5 minutes
			}

			populateOTEMetadata(testCase, tt.extensionResult)

			if testCase.Lifecycle != tt.expectedLifecycle {
				t.Errorf("Lifecycle = %q, want %q", testCase.Lifecycle, tt.expectedLifecycle)
			}
			if testCase.StartTime != tt.expectedStartTime {
				t.Errorf("StartTime = %q, want %q", testCase.StartTime, tt.expectedStartTime)
			}
			if testCase.EndTime != tt.expectedEndTime {
				t.Errorf("EndTime = %q, want %q", testCase.EndTime, tt.expectedEndTime)
			}
			if testCase.SourceImage != tt.expectedSourceImage {
				t.Errorf("SourceImage = %q, want %q", testCase.SourceImage, tt.expectedSourceImage)
			}
			if testCase.SourceBinary != tt.expectedSourceBinary {
				t.Errorf("SourceBinary = %q, want %q", testCase.SourceBinary, tt.expectedSourceBinary)
			}
			if testCase.SourceURL != tt.expectedSourceURL {
				t.Errorf("SourceURL = %q, want %q", testCase.SourceURL, tt.expectedSourceURL)
			}
			if testCase.SourceCommit != tt.expectedSourceCommit {
				t.Errorf("SourceCommit = %q, want %q", testCase.SourceCommit, tt.expectedSourceCommit)
			}
		})
	}
}

func Test_junitXMLWithOTEAttributes(t *testing.T) {
	startTime := time.Date(2023, 12, 25, 10, 0, 0, 0, time.UTC)
	endTime := time.Date(2023, 12, 25, 10, 5, 0, 0, time.UTC)

	// Create a JUnit test case and populate it with OTE metadata
	extensionResult := &extensions.ExtensionTestResult{
		ExtensionTestResult: &extensiontests.ExtensionTestResult{
			Name:      "example-test",
			Lifecycle: extensiontests.LifecycleBlocking,
			StartTime: dbtime.Ptr(startTime),
			EndTime:   dbtime.Ptr(endTime),
		},
		Source: extensions.Source{
			Source: &extension.Source{
				Commit:    "abc123def456789",
				SourceURL: "https://github.com/openshift/origin",
			},
			SourceImage:  "tests",
			SourceBinary: "openshift-tests",
		},
	}

	junitTestCase := &junitapi.JUnitTestCase{
		Name:     "example-test",
		Duration: 300.0, // 5 minutes
	}

	// Populate the OTE metadata
	populateOTEMetadata(junitTestCase, extensionResult)

	// Create a test suite containing our test case
	suite := &junitapi.JUnitTestSuite{
		Name:      "test-suite",
		NumTests:  1,
		Duration:  300.0,
		TestCases: []*junitapi.JUnitTestCase{junitTestCase},
	}

	// Verify OTE metadata is present
	if junitTestCase.Lifecycle != "blocking" {
		t.Errorf("Lifecycle = %q, want %q", junitTestCase.Lifecycle, "blocking")
	}
	if junitTestCase.StartTime != "2023-12-25T10:00:00Z" {
		t.Errorf("StartTime = %q, want %q", junitTestCase.StartTime, "2023-12-25T10:00:00Z")
	}
	if junitTestCase.EndTime != "2023-12-25T10:05:00Z" {
		t.Errorf("EndTime = %q, want %q", junitTestCase.EndTime, "2023-12-25T10:05:00Z")
	}
	if junitTestCase.SourceImage != "tests" {
		t.Errorf("SourceImage = %q, want %q", junitTestCase.SourceImage, "tests")
	}
	if junitTestCase.SourceBinary != "openshift-tests" {
		t.Errorf("SourceBinary = %q, want %q", junitTestCase.SourceBinary, "openshift-tests")
	}
	if junitTestCase.SourceURL != "https://github.com/openshift/origin" {
		t.Errorf("SourceURL = %q, want %q", junitTestCase.SourceURL, "https://github.com/openshift/origin")
	}
	if junitTestCase.SourceCommit != "abc123def456789" {
		t.Errorf("SourceCommit = %q, want %q", junitTestCase.SourceCommit, "abc123def456789")
	}

	// Verify XML marshaling includes the new attributes
	xmlData, err := xml.MarshalIndent(suite, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal XML: %v", err)
	}

	xmlString := string(xmlData)

	// Check that our custom attributes are in the XML
	expectedAttributes := []string{
		`lifecycle="blocking"`,
		`start-time="2023-12-25T10:00:00Z"`,
		`end-time="2023-12-25T10:05:00Z"`,
		`source-image="tests"`,
		`source-binary="openshift-tests"`,
		`source-url="https://github.com/openshift/origin"`,
		`source-commit="abc123def456789"`,
	}

	for _, attr := range expectedAttributes {
		if !strings.Contains(xmlString, attr) {
			t.Errorf("XML does not contain expected attribute: %s\nXML output:\n%s", attr, xmlString)
		}
	}
}
