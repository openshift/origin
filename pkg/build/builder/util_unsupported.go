// +build !linux,!darwin

package builder

import (
	"errors"

	s2iapi "github.com/openshift/source-to-image/pkg/api"
)

// getContainerNetworkConfig determines whether the builder is running as a container
// by examining /proc/self/cgroup. This context is then passed to source-to-image.
// It returns a suitable argument for NetworkMode.  If the container platform is
// CRI-O, it also returns a path for /etc/resolv.conf, suitable for bindmounting.
func getContainerNetworkConfig() (string, string, error) {
	return "", "", errors.New("getContainerNetworkConfig is unsupported on this platform")
}

// GetCGroupLimits returns a struct populated with cgroup limit values gathered
// from the local /sys/fs/cgroup filesystem.  Overflow values are set to
// math.MaxInt64.
func GetCGroupLimits() (*s2iapi.CGroupLimits, error) {
	return nil, errors.New("GetCGroupLimits is unsupported on this platform")
}

// getCgroupParent determines the parent cgroup for a container from
// within that container.
func getCgroupParent() (string, error) {
	return "", errors.New("getCgroupParent is unsupported on this platform")
}
