package utils

import (
	"fmt"
	"golang.org/x/sys/unix"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/docker/docker/pkg/mount"
)

// GetDiskUsageStats accepts a path to a directory or file
// and returns the number of bytes and inodes used by the path
func GetDiskUsageStats(path string) (uint64, uint64, error) {
	var dirSize, inodeCount uint64

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		fileStat, error := os.Lstat(path)
		if error != nil {
			if fileStat.Mode()&os.ModeSymlink != 0 {
				// Is a symlink; no error should be returned
				return nil
			}
			return error
		}

		dirSize += uint64(info.Size())
		inodeCount++

		return nil
	})

	if err != nil {
		return 0, 0, err
	}

	return dirSize, inodeCount, err
}

// GetDeviceUUIDFromPath accepts a path, and will find the device
// corresponding to the path and return the UUID of that device
func GetDeviceUUIDFromPath(devicePath string) (string, error) {
	const dir = "/dev/disk/by-uuid"

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", nil
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		path := filepath.Join(dir, file.Name())
		target, err := os.Readlink(path)
		if err != nil {
			continue
		}
		device, err := filepath.Abs(filepath.Join(dir, target))
		if err != nil {
			return "", fmt.Errorf("failed to resolve the absolute path of %q", filepath.Join(dir, target))
		}
		if strings.Compare(device, devicePath) == 0 {
			return file.Name(), nil
		}
	}

	return "", fmt.Errorf("device path %s not found", devicePath)
}

// GetStattFromPath is a helper function that returns the Stat_t
// object for a given path
func GetStattFromPath(path string) (syscall.Stat_t, error) {
	statInfo := syscall.Stat_t{}
	err := syscall.Lstat(path, &statInfo)
	if err != nil {
		return statInfo, err
	}
	return statInfo, nil
}

// GetDeviceNameFromPath iterates through the mounts and matches
// the one that the provided path is on
func GetDeviceNameFromPath(path string) (string, error) {
	statInfo, err := GetStattFromPath(path)
	if err != nil {
		return "", err
	}

	mounts, err := mount.GetMounts()
	if err != nil {
		return "", err
	}

	queryMajor := int(unix.Major(uint64(statInfo.Dev)))
	queryMinor := int(unix.Minor(uint64(statInfo.Dev)))

	for _, mount := range mounts {
		if mount.Minor == queryMinor && mount.Major == queryMajor {
			return mount.Source, nil
		}
	}

	return "", fmt.Errorf("no match found")
}
