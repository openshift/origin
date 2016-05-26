package spdystream

import (
	"fmt"

	"github.com/golang/glog"
)

func debugMessage(fmtDirective string, args ...interface{}) {
	fmt.Printf(fmtDirective+"\n", args...)
	glog.V(1).Infof(fmtDirective+"\n", args...)
}
