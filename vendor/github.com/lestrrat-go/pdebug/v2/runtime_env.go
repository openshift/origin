// +build debug

package pdebug

import (
	"os"
	"strconv"
)

// This is protected by the "debug" build tag -- because we only want to
// ever parse the environment variable when the debug code is enabled.
// When debug0 tag is specified, this variable is ALWAYS true.
// When debug  tag is specified, this variable starts out as false, but
// can be toggled to true when the environment variable PDEBUG_TRACE is specified.
// When no tag is specified, this variable isn't event compiled in
var Trace bool

func init() {
	if b, err := strconv.ParseBool(os.Getenv("PDEBUG_TRACE")); err == nil && b {
		Trace = true
	}
}
