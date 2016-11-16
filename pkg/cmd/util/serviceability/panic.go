package serviceability

import (
	"strings"
	"time"

	"github.com/golang/glog"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
)

// BehaviorOnPanic is a helper for setting the crash mode of OpenShift when a panic is caught.
// It returns a function that should be the defer handler for the caller.
func BehaviorOnPanic(mode string) (fn func()) {
	fn = func() {}
	switch {
	case mode == "crash":
		glog.Infof("Process will terminate as soon as a panic occurs.")
		utilruntime.ReallyCrash = true
	case strings.HasPrefix(mode, "sentry:"):
		url := strings.TrimPrefix(mode, "sentry:")
		m, err := NewSentryMonitor(url)
		if err != nil {
			glog.Errorf("Unable to start Sentry for panic tracing: %v", err)
			return
		}
		glog.Infof("Process will log all panics and errors to Sentry.")
		utilruntime.ReallyCrash = false
		utilruntime.PanicHandlers = append(utilruntime.PanicHandlers, m.CapturePanic)
		utilruntime.ErrorHandlers = append(utilruntime.ErrorHandlers, m.CaptureError)
		fn = func() {
			if r := recover(); r != nil {
				m.CapturePanicAndWait(r, 2*time.Second)
				panic(r)
			}
		}
	case len(mode) == 0:
		// default panic behavior
		utilruntime.ReallyCrash = false
	default:
		glog.Errorf("Unrecognized panic behavior")
	}
	return
}
