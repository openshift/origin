package filters

import (
	"context"
	"testing"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/origin/pkg/test/extensions"
)

func TestQualifiersFilter(t *testing.T) {
	testCases := []struct {
		name       string
		qualifiers []string
		tests      extensions.ExtensionTestSpecs
		expected   []string
	}{
		{
			name:       "no qualifiers - pass all tests through",
			qualifiers: nil,
			tests: extensions.ExtensionTestSpecs{
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test1"}},
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test2"}},
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test3"}},
			},
			expected: []string{"test1", "test2", "test3"},
		},
		{
			name:       "empty qualifiers - pass all tests through",
			qualifiers: []string{},
			tests: extensions.ExtensionTestSpecs{
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test1"}},
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test2"}},
			},
			expected: []string{"test1", "test2"},
		},
		{
			name:       "filter by name contains",
			qualifiers: []string{`name.contains("api")`},
			tests: extensions.ExtensionTestSpecs{
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test api functionality"}},
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test ui functionality"}},
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "api server test"}},
			},
			expected: []string{"test api functionality", "api server test"},
		},
		{
			name:       "filter by suite tag",
			qualifiers: []string{`name.contains("[Suite:openshift/conformance")`},
			tests: extensions.ExtensionTestSpecs{
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test1 [Suite:openshift/conformance/parallel]"}},
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test2 [Suite:openshift/disruptive]"}},
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test3 [Suite:openshift/conformance/serial]"}},
			},
			expected: []string{"test1 [Suite:openshift/conformance/parallel]", "test3 [Suite:openshift/conformance/serial]"},
		},
		{
			name:       "filter by early tests",
			qualifiers: []string{`name.contains("[Early]")`},
			tests: extensions.ExtensionTestSpecs{
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test1 [Early] [Suite:openshift/conformance]"}},
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test2 [Late] [Suite:openshift/conformance]"}},
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test3 [Suite:openshift/conformance]"}},
			},
			expected: []string{"test1 [Early] [Suite:openshift/conformance]"},
		},
		{
			name:       "qualifiers are OR'd",
			qualifiers: []string{`name.contains("[Early]")`, `name.contains("[Late]")`},
			tests: extensions.ExtensionTestSpecs{
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test1 [Early] [Suite:openshift/conformance]"}},
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test2 [Late] [Suite:openshift/conformance]"}},
				&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test3 [Suite:openshift/conformance]"}},
			},
			expected: []string{"test1 [Early] [Suite:openshift/conformance]", "test2 [Late] [Suite:openshift/conformance]"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewQualifiersFilter(tc.qualifiers)

			result, err := filter.Filter(context.Background(), tc.tests)

			require.NoError(t, err)
			assert.Len(t, result, len(tc.expected))
			for i, expectedName := range tc.expected {
				assert.Equal(t, expectedName, result[i].Name)
			}
		})
	}
}

func TestQualifiersFilterShouldApply(t *testing.T) {
	testCases := []struct {
		name       string
		qualifiers []string
		expected   bool
	}{
		{
			name:       "nil qualifiers",
			qualifiers: nil,
			expected:   false,
		},
		{
			name:       "empty qualifiers",
			qualifiers: []string{},
			expected:   false,
		},
		{
			name:       "with qualifiers",
			qualifiers: []string{`name.contains("test")`},
			expected:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewQualifiersFilter(tc.qualifiers)
			assert.Equal(t, tc.expected, filter.ShouldApply())
		})
	}
}

func TestQualifiersFilterErrorHandling(t *testing.T) {
	// Test with invalid CEL expression
	filter := NewQualifiersFilter([]string{`invalid.cel.expression(`})

	tests := extensions.ExtensionTestSpecs{
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test1"}},
	}

	result, err := filter.Filter(context.Background(), tests)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to filter tests by qualifiers")
}
