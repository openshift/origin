package node

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang/glog"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

// TODO this is a best effort check at the moment that should either move to kubelet or be removed entirely
// EnsureKubeletAccess performs a number of test operations that the Kubelet requires to properly function.
// All errors here are fatal.
func EnsureKubeletAccess() {
	if cmdutil.Env("OPENSHIFT_CONTAINERIZED", "") == "true" {
		if _, err := os.Stat("/rootfs"); os.IsPermission(err) || os.IsNotExist(err) {
			glog.Fatal("error: Running in containerized mode, but cannot find the /rootfs directory - be sure to mount the host filesystem at /rootfs (read-only) in the container.")
		}
		if !sameFileStat(true, "/rootfs/sys", "/sys") {
			glog.Fatal("error: Running in containerized mode, but the /sys directory in the container does not appear to match the host /sys directory - be sure to mount /sys into the container.")
		}
		if !sameFileStat(true, "/rootfs/var/run", "/var/run") {
			glog.Fatal("error: Running in containerized mode, but the /var/run directory in the container does not appear to match the host /var/run directory - be sure to mount /var/run (read-write) into the container.")
		}
	}
	// TODO: check whether we can mount disks (for volumes)
	// TODO: check things cAdvisor needs to properly function
	// TODO: test a cGroup move?
}

// sameFileStat checks whether the provided paths are the same file, to verify that a user has correctly
// mounted those binaries
func sameFileStat(requireMode bool, src, dst string) bool {
	srcStat, err := os.Stat(src)
	if err != nil {
		glog.V(4).Infof("Unable to stat %q: %v", src, err)
		return false
	}
	dstStat, err := os.Stat(dst)
	if err != nil {
		glog.V(4).Infof("Unable to stat %q: %v", dst, err)
		return false
	}
	if requireMode && srcStat.Mode() != dstStat.Mode() {
		glog.V(4).Infof("Mode mismatch between %q (%s) and %q (%s)", src, srcStat.Mode(), dst, dstStat.Mode())
		return false
	}
	if !os.SameFile(srcStat, dstStat) {
		glog.V(4).Infof("inode and device mismatch between %q (%s) and %q (%s)", src, srcStat, dst, dstStat)
		return false
	}
	return true
}

// TODO we need to stop doing this or get it upstream
// EnsureVolumeDir attempts to convert the provided volume directory argument to
// an absolute path and create the directory if it does not exist. Will exit if
// an error is encountered.
func EnsureVolumeDir(volumeDirName string) {
	if err := initializeVolumeDir(volumeDirName); err != nil {
		glog.Fatal(err)
	}
}

func initializeVolumeDir(rootDirectory string) error {
	if !filepath.IsAbs(rootDirectory) {
		return fmt.Errorf("%q is not an absolute path", rootDirectory)
	}

	if _, err := os.Stat(rootDirectory); os.IsNotExist(err) {
		if err := os.MkdirAll(rootDirectory, 0750); err != nil {
			return fmt.Errorf("Couldn't create kubelet volume root directory '%s': %s", rootDirectory, err)
		}
	}
	return nil
}
