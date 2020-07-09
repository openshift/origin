package proc

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"k8s.io/klog"
)

// parseProcForZombies parses the current procfs mounted at /proc
// to find processes in the zombie state.
func parseProcForZombies() ([]int, error) {
	files, err := ioutil.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var zombies []int
	for _, file := range files {
		processID, err := strconv.Atoi(file.Name())
		if err != nil {
			break
		}
		stateFilePath := filepath.Join("/proc", file.Name(), "status")
		fd, err := os.Open(stateFilePath)
		if err != nil {
			klog.Errorf("Failed to open %v for getting process status: %v", stateFilePath, err)
			continue
		}
		defer fd.Close()
		fs := bufio.NewScanner(fd)
		for fs.Scan() {
			line := fs.Text()
			if strings.HasPrefix(line, "State:") {
				if strings.Contains(line, "zombie") {
					zombies = append(zombies, processID)
				}
				break
			}
		}
	}

	return zombies, nil
}

// StartReaper starts a goroutine to reap processes periodically if called
// from a pid 1 process.
// If period is 0, then it is defaulted to 5 seconds.
// A caller can adjust the period depending on how many and how frequently zombie
// processes are created and need to be reaped.
func StartReaper(period time.Duration) {
	if os.Getpid() == 1 {
		const defaultReaperPeriodSeconds = 5
		if period == 0 {
			period = defaultReaperPeriodSeconds * time.Second
		}
		go func() {
			var zs []int
			var err error
			for {
				zs, err = parseProcForZombies()
				if err != nil {
					klog.Errorf("Failed to parse proc filesystem to find processes to reap: %v", err)
					continue
				}
				time.Sleep(period)
				for _, z := range zs {
					cpid, err := syscall.Wait4(z, nil, syscall.WNOHANG, nil)
					if err != nil {
						klog.Errorf("Unable to reap process pid %v: %v", cpid, err)
					}
				}
			}
		}()
	}
}
