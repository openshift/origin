package ginkgo

import (
	"encoding/xml"
	"testing"
	"time"

	"github.com/openshift-eng/openshift-tests-extension/pkg/dbtime"
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
		name              string
		extensionResult   *extensions.ExtensionTestResult
		expectedLifecycle string
		expectedStartTime string
		expectedEndTime   string
	}{
		{
			name:              "nil extension result",
			extensionResult:   nil,
			expectedLifecycle: "",
			expectedStartTime: "",
			expectedEndTime:   "",
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
			},
			expectedLifecycle: "blocking",
			expectedStartTime: "2023-12-25T10:00:00Z",
			expectedEndTime:   "2023-12-25T10:05:00Z",
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
			},
			expectedLifecycle: "informing",
			expectedStartTime: "2023-12-25T10:00:00Z",
			expectedEndTime:   "2023-12-25T10:05:00Z",
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
			expectedLifecycle: "blocking",
			expectedStartTime: "",
			expectedEndTime:   "",
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
	}

	for _, attr := range expectedAttributes {
		if !contains(xmlString, attr) {
			t.Errorf("XML does not contain expected attribute: %s\nXML output:\n%s", attr, xmlString)
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}()
}
