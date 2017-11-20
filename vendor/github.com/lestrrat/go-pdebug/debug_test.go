//+build debug,!debug0

package pdebug

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDebugEnabled(t *testing.T) {
	if !assert.True(t, Enabled, "Enable is true") {
		return
	}

	b, err := strconv.ParseBool(os.Getenv("PDEBUG_TRACE"))
	if err == nil && b {
		if !assert.True(t, Trace, "Trace is true") {
			return
		}
		t.Logf("Trace is enabled")
	} else {
		if !assert.False(t, Trace, "Trace is false") {
			return
		}
		t.Logf("Trace is disabled")
	}
}