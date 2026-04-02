package extensions

import (
	"testing"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestViewerHTMLTemplateIsRenderable(t *testing.T) {
	// This test ensures the viewer.html template can be parsed by Go's template engine.
	// It catches issues like unescaped {{ }} sequences in comments or other content
	// that would cause the template to fail to parse.

	testData := []byte(`[{"name":"test","result":"passed"}]`)
	suiteName := "test-suite"

	html, err := extensiontests.RenderResultsHTML(testData, suiteName)
	require.NoError(t, err, "viewer.html template should be parseable")
	assert.Contains(t, string(html), "test-suite", "rendered HTML should contain suite name")
	assert.Contains(t, string(html), `"name":"test"`, "rendered HTML should contain test data")
}

func TestToHTMLModes(t *testing.T) {
	// Test that both HTML output modes work correctly
	results := ExtensionTestResults{
		&ExtensionTestResult{
			ExtensionTestResult: &extensiontests.ExtensionTestResult{
				Name:   "passing-test",
				Result: extensiontests.ResultPassed,
				Output: "some output",
			},
		},
		&ExtensionTestResult{
			ExtensionTestResult: &extensiontests.ExtensionTestResult{
				Name:   "failing-test",
				Result: extensiontests.ResultFailed,
				Output: "failure output",
				Error:  "error message",
			},
		},
	}

	t.Run("summary mode", func(t *testing.T) {
		html, err := results.ToHTML("test-suite", HTMLOutputSummary)
		require.NoError(t, err)
		assert.NotEmpty(t, html)
		// Summary mode should still contain the test names
		assert.Contains(t, string(html), "passing-test")
		assert.Contains(t, string(html), "failing-test")
	})

	t.Run("everything mode", func(t *testing.T) {
		html, err := results.ToHTML("test-suite", HTMLOutputEverything)
		require.NoError(t, err)
		assert.NotEmpty(t, html)
		// Everything mode should contain all output
		assert.Contains(t, string(html), "some output")
		assert.Contains(t, string(html), "failure output")
	})
}
