package gochecksumtype

import (
	"flag"
)

// bit-flag set.
type cfg uint8

const (
	// Whether a default signifies exhaustiveness.
	defaultExhaustive       cfg = 1 << 0
	includeSharedInterfaces cfg = 1 << 1
)

func (c cfg) defaultSignifiesExhaustive() bool {
	return c&defaultExhaustive != 0
}
func (c cfg) includeSharedInterfaces() bool {
	return c&includeSharedInterfaces != 0
}

func cfgFromFlags(flags flag.FlagSet) (result cfg) {
	if getBoolFlag(flags, flagDefaultSignifiesExhaustive) {
		result |= defaultExhaustive
	}
	if getBoolFlag(flags, flagIncludeSharedInterfaces) {
		result |= includeSharedInterfaces
	}
	return result
}

func getBoolFlag(flags flag.FlagSet, name string) bool {
	f := flags.Lookup(name).Value.(flag.Getter)
	return f != nil && f.Get().(bool)
}

const (
	flagDefaultSignifiesExhaustive = "default-signifies-exhaustive"
	flagIncludeSharedInterfaces    = "include-shared-interfaces"
)

// checks exhaustiveness of sum type
// switch statements. Sum types are declared with a //sumtype:decl comment
// above a sealed interface type.
func newFlags() (fs flag.FlagSet) {
	fs.Bool(flagDefaultSignifiesExhaustive, true,
		"Presence of a non-panicking default case satisfies exhaustiveness.")
	fs.Bool(flagIncludeSharedInterfaces, false,
		"Include shared interfaces in the exhaustiveness check.")
	return fs
}
