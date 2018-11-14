// +build linux

package common

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/golang/glog"
)

func InitVariableLogging() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGUSR1)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		// The first time logging is changed, jump to V(6)
		var newLevel int32 = 5

		wg.Done()
		for {
			<-c
			newLevel++
			// Loop between V(6) and V(1)
			if newLevel >= 7 {
				newLevel = 1
			}

			var level glog.Level
			if err := level.Set(fmt.Sprintf("%d", newLevel)); err != nil {
				glog.Errorf("failed set glog.logging.verbosity %d: %v", newLevel, err)
			} else {
				glog.Infof("set glog.logging.verbosity to %d", newLevel)
			}
		}
	}()
	wg.Wait()
}
