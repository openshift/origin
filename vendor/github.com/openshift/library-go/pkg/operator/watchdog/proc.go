package watchdog

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"syscall"
)

// FindProcessByName find the process name specified by name and return the PID of that process.
// If the process is not found, the bool is false.
// NOTE: This require container with shared process namespace (if run as side-car).
func FindProcessByName(name string) (int, bool, error) {
	files, err := ioutil.ReadDir("/proc")
	if err != nil {
		return 0, false, err
	}
	// sort means we start with the directories with numbers
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		// only scan process directories (eg. /proc/1234)
		pid, err := strconv.Atoi(file.Name())
		if err != nil {
			continue
		}
		// read the /proc/123/exe symlink that points to a process
		linkTarget := readlink(filepath.Join("/proc", file.Name(), "exe"))
		if path.Base(linkTarget) != name {
			continue
		}
		return pid, true, nil
	}
	return 0, false, nil
}

// ProcessExists checks if the process specified by a PID exists in the /proc filesystem.
// Error is returned when the stat on the /proc dir fail (permission issue).
func ProcessExists(pid int) (bool, error) {
	procDir, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if procDir.IsDir() {
		return true, nil
	} else {
		return false, fmt.Errorf("unexpected error: /proc/%d is file, not directory", pid)
	}
}

// readlink is copied from the os.Readlink() but does not return error when the target path does not exists.
// This is used to read broken links as in case of share PID namespace, the /proc/1/exe points to a binary
// that does not exists from the source container.
func readlink(name string) string {
	for l := 128; ; l *= 2 {
		b := make([]byte, l)
		n, _ := syscall.Readlink(name, b)
		if n < 0 {
			n = 0
		}
		if n < l {
			return string(b[0:n])
		}
	}
}
