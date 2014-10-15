package flagtypes

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

// StringList is a type overriding util.StringList to provide the Type() method in order to fulfill the spf13/pflags.Value interface.
// It is a util.StringList in order to be compatible with util.CompileRegexps.  util.CompileRegexps should probably be modified
// to accept a direct []string to make it more easily reusable, but that has to happen first and then this code can be tidied up.
type StringList util.StringList

// String returns the string representation of the StringList
func (sl *StringList) String() string {
	return fmt.Sprint(*sl)
}

// Set takes a string, splits it on commas, ensures there are no empty parts of the split, and appends them to the receiver.
func (sl *StringList) Set(value string) error {
	*sl = []string{}
	for _, s := range strings.Split(value, ",") {
		if len(s) == 0 {
			return fmt.Errorf("value should not be an empty string")
		}
		*sl = append(*sl, s)
	}
	return nil
}

// Type returns a string representation of what kind of argument this is
func (sl *StringList) Type() string {
	return "cmd.flagtypes.StringList"
}
