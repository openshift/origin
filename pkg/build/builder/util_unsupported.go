// +build !linux

package builder

import "errors"

// getCgroupParent determines the parent cgroup for a container from
// within that container.
func getCgroupParent() (string, error) {
	return "", errors.New("getCgroupParent is unsupported on this platform")
}
