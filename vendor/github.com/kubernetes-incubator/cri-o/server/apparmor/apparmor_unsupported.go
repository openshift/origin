// +build !apparmor

package apparmor

// IsEnabled returns false, when build without apparmor build tag.
func IsEnabled() bool {
	return false
}

// EnsureDefaultApparmorProfile dose nothing, when build without apparmor build tag.
func EnsureDefaultApparmorProfile() error {
	return nil
}
