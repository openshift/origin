package serviceability

import (
	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

// BehaviorOnPanic is a helper for setting the crash mode of OpenShift when a panic is caught.
func BehaviorOnPanic(mode string) {
	switch mode {
	case "crash":
		glog.V(4).Infof("OpenShift will terminate as soon as a panic occurs.")
		util.ReallyCrash = true
	}
}
