package filters

import (
	"context"
	"testing"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/origin/pkg/test/extensions"
)

func TestKubeRebaseFilter(t *testing.T) {
	// Test with nil config (should pass all tests through)
	filter := NewKubeRebaseTestsFilter(nil)

	tests := extensions.ExtensionTestSpecs{
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "normal test"}},
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "[sig-api-machinery] health handlers should contain necessary checks"}},
	}

	result, err := filter.Filter(context.Background(), tests)

	require.NoError(t, err)
	assert.Len(t, result, 2) // All tests should pass through when restConfig is nil
	assert.Equal(t, "normal test", result[0].Name)
	assert.Equal(t, "[sig-api-machinery] health handlers should contain necessary checks", result[1].Name)
}
