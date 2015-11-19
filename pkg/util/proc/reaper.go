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
					cpid, _ := syscall.Wait4(-1, nil, syscall.WNOHANG, nil)
					if cpid < 1 {
						break
					}

					glog.V(4).Infof("Reaped process with pid %d", cpid)
				}
			}
		}()
	}
}
