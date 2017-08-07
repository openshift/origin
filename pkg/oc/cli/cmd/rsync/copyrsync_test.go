package rsync

import (
	"io/ioutil"
	"testing"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"
)

// rshAllowedFlags is the set of flags in the rsync command that
// can be passed to the rsh command
var rshAllowedFlags = sets.NewString("container")

// TestRshExcludeFlags ensures that only rsync flags that are allowed to be set on the rsh flag
// are not included in the rshExcludeFlags set.
func TestRshExcludeFlags(t *testing.T) {
	rsyncCmd := NewCmdRsync("rsync", "oc", nil, ioutil.Discard, ioutil.Discard)
	rsyncCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if !rshExcludeFlags.Has(flag.Name) && !rshAllowedFlags.Has(flag.Name) {
			t.Errorf("Unknown flag %s. Please add to rshExcludeFlags or to rshAllowedFlags", flag.Name)
		}
	})
}

// TestRsyncEscapeCommand ensures that command line options supplied to oc rsync
// are properly escaped.
func TestRsyncEscapeCommand(t *testing.T) {
	// the strings to run through rsyncEscapeCommand
	stringsToEscape := []string{
		`thisshouldnotgetescapedorquoted`,
		`this should get quoted for spaces`,
		`this" should get escaped and quoted`,
		`"this should get escaped and quoted"`,
		`this\ should get quoted`,
		`this' should get quoted`,
	}
	// this is how the strings should be escaped by rsyncEscapeCommand
	stringsShouldMatch := []string{
		`thisshouldnotgetescapedorquoted`,
		`"this should get quoted for spaces"`,
		`"this"" should get escaped and quoted"`,
		`"""this should get escaped and quoted"""`,
		`"this\ should get quoted"`,
		`"this' should get quoted"`,
	}

	escapedStrings := rsyncEscapeCommand(stringsToEscape)

	for key, val := range escapedStrings {
		if val != stringsShouldMatch[key] {
			t.Errorf("%v did not match %v", val, stringsShouldMatch[key])
		}
	}
}
