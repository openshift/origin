// Package config provides helpers for handling revive configuration and rule option names.
package config

import "strings"

// NormalizeOption returns an option name lowercased and without hyphens, so that the camelCase, kebab-case,
// and lowercase spellings of an option all map to the same value.
//
// Example: NormalizeOption("allowTypesBefore"), NormalizeOption("allow-types-before") -> "allowtypesbefore".
func NormalizeOption(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "-", ""))
}
