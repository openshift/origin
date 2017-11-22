//+build !debug,!debug0

package pdebug

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDisabled(t *testing.T) {
	assert.False(t, Enabled, "Enable is false")
	assert.False(t, Trace, "Trace is false")
}