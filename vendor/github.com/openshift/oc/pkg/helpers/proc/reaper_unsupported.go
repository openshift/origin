// +build !linux

package proc

// StartReaper has no effect on non-linux platforms.
// Support for other unices will be added.
func StartReaper() {
}
