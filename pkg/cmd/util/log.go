package util

import "github.com/golang/glog"

// GetLogLevel returns the current glog log level
func GetLogLevel() (level int) {
	for level = 5; level >= 0; level-- {
		if glog.V(glog.Level(level)) == true {
			break
		}
	}
	return
}
