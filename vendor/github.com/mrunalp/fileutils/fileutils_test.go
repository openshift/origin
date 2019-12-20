package fileutils

import (
	"os"
	"syscall"
	"testing"
)

type device struct {
	device string
	major  uint64
	minor  uint64
}

func TestDeviceNumbers(t *testing.T) {
	devices := []device{
		{
			device: "/dev/mem",
			major:  1,
			minor:  1,
		},
		{
			device: "/dev/urandom",
			major:  1,
			minor:  9,
		},
		{
			device: "/dev/tty0",
			major:  4,
			minor:  0,
		},
		{
			device: "/dev/tty",
			major:  5,
			minor:  0,
		},
	}

	for _, device := range devices {
		si, err := os.Lstat(device.device)
		if err != nil {
			t.Errorf("not able to Lstat the device: %v", err)
		}

		st, ok := si.Sys().(*syscall.Stat_t)

		if !ok {
			t.Errorf("could not convert to syscall.Stat_t")
		}

		internalDevice := uint64(st.Rdev)

		givenMajor := major(internalDevice)
		if givenMajor != device.major {
			t.Errorf("%s major not matching - expected: %d - given: %d", device.device, device.major, givenMajor)
		}

		givenMinor := minor(internalDevice)
		if givenMinor != device.minor {
			t.Errorf("%s minor not matching - expected: %d - given: %d", device.device, device.minor, givenMinor)
		}

		givenMkdev := mkdev(int64(device.major), int64(device.minor))
		if givenMkdev != uint32(internalDevice) {
			t.Errorf("%s mkdev not matching - expected: %d - given: %d", device.device, uint32(internalDevice), givenMkdev)
		}
	}
}
