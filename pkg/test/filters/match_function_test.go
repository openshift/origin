package filters

import (
	"context"
	"testing"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/origin/pkg/test/extensions"
)

func TestSuiteMatcherFilter(t *testing.T) {
	// Test with nil matcher (should pass all tests through)
	filter := NewMatchFnFilter(nil)

	tests := extensions.ExtensionTestSpecs{
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test1"}},
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test2"}},
	}

	result, err := filter.Filter(context.Background(), tests)

	require.NoError(t, err)
	assert.Len(t, result, 2) // All tests should pass through when suite is nil
	assert.Equal(t, "test1", result[0].Name)
	assert.Equal(t, "test2", result[1].Name)
}
