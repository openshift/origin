//+build debug0,!debug

package pdebug

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDebug0Enabled(t *testing.T) {
	assert.True(t, Enabled, "Enable is true")
}
