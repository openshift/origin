package variable

import (
	"fmt"
	"os"
	"strings"

	"github.com/openshift/origin/pkg/version"
)

// KeyFunc returns the value associated with the provided key or false if no
// such key exists.
type KeyFunc func(key string) (string, bool)

// Expand expands a string and ignores any errors that occur - keys that are
// not recognized are replaced with the empty string.
func Expand(s string, fns ...KeyFunc) string {
	val, _ := ExpandStrict(s, append(fns, Empty)...)
	return val
}

// ExpandStrict expands a string using a series of common format functions
func ExpandStrict(s string, fns ...KeyFunc) (string, error) {
	unmatched := []string{}
	result := os.Expand(s, func(key string) string {
		for _, fn := range fns {
			val, ok := fn(key)
			if !ok {
				continue
			}
			return val
		}
		unmatched = append(unmatched, key)
		return ""
	})

	switch len(unmatched) {
	case 0:
		return result, nil
	case 1:
		return "", fmt.Errorf("the key %q in %q is not recognized", unmatched[0], s)
	default:
		return "", fmt.Errorf("multiple keys in %q were not recognized: %s", s, strings.Join(unmatched, ", "))
	}
}

// Empty is a KeyFunc which always returns true and the empty string.
func Empty(s string) (string, bool) {
	return "", true
}

// Identity is a KeyFunc that returns the same format rules.
func Identity(key string) (string, bool) {
	return fmt.Sprintf("${%s}", key), true
}

// Versions is a KeyFunc for retrieving information about the current version.
func Versions(key string) (string, bool) {
	switch key {
	case "shortcommit":
		s := overrideVersion.GitCommit
		if len(s) > 7 {
			s = s[:7]
		}
		return s, true
	case "version":
		s := overrideVersion.LastSemanticVersion()
		return s, true
	default:
		return "", false
	}
}

// Env is a KeyFunc which always returns a string
func Env(key string) (string, bool) {
	return os.Getenv(key), true
}

// overrideVersion is the latest version, exposed for testing.
var overrideVersion = version.Get()
