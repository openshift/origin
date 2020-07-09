// +build !linux

package proc

import "time"

// StartReaper has no effect on non-linux platforms.
// Support for other unices will be added.
func StartReaper(period time.Duration) {
}
