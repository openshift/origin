// +build linux

package proc

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
)

// StartReaper starts a goroutine to reap processes if called from a process
// that has pid 1.
func StartReaper() {
	if os.Getpid() == 1 {
		glog.V(4).Infof("Launching reaper")
		go func() {
			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGCHLD)
			for {
				// Wait for a child to terminate
				sig := <-sigs
				glog.V(4).Infof("Signal received: %v", sig)
				for {
					// Reap processes
					glog.V(4).Infof("Waiting to reap")
					cpid, err := syscall.Wait4(-1, nil, 0, nil)

					// Break out if there are no more processes to reap
					if err == syscall.ECHILD {
						glog.V(4).Infof("Received: %v", err)
						break
					}

					glog.V(4).Infof("Reaped process with pid %d", cpid)
				}
			}
		}()
	}
}
