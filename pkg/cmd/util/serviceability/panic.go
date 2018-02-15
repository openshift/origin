package serviceability

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// BehaviorOnPanic is a helper for setting the crash mode of OpenShift when a panic is caught.
// It returns a function that should be the defer handler for the caller.
func BehaviorOnPanic(modeString string) func() {
	modes := []string{}
	if err := json.Unmarshal([]byte(modeString), &modes); err != nil {
		return behaviorOnPanic(modeString)
	}

	fns := []func(){}

	for _, mode := range modes {
		fns = append(fns, behaviorOnPanic(mode))
	}

	return func() {
		for _, fn := range fns {
			fn()
		}
	}
}

func behaviorOnPanic(mode string) func() {
	doNothing := func() {}

	switch {
	case mode == "crash":
		glog.Infof("Process will terminate as soon as a panic occurs.")
		utilruntime.ReallyCrash = true
		return doNothing

	case strings.HasPrefix(mode, "crash-after-delay:"):
		delayDurationString := strings.TrimPrefix(mode, "crash-after-delay:")
		delayDuration, err := time.ParseDuration(delayDurationString)
		if err != nil {
			glog.Errorf("Unable to start crash-after-delay.  Crashing immediately instead: %v", err)
			utilruntime.ReallyCrash = true
			return doNothing
		}
		glog.Infof("Process will terminate %v after a panic occurs.", delayDurationString)
		utilruntime.ReallyCrash = false
		utilruntime.PanicHandlers = append(utilruntime.PanicHandlers, crashOnDelay(delayDuration, delayDurationString))
		return doNothing

	case strings.HasPrefix(mode, "sentry:"):
		url := strings.TrimPrefix(mode, "sentry:")
		m, err := NewSentryMonitor(url)
		if err != nil {
			glog.Errorf("Unable to start Sentry for panic tracing: %v", err)
			return doNothing
		}
		glog.Infof("Process will log all panics and errors to Sentry.")
		utilruntime.ReallyCrash = false
		utilruntime.PanicHandlers = append(utilruntime.PanicHandlers, m.CapturePanic)
		utilruntime.ErrorHandlers = append(utilruntime.ErrorHandlers, m.CaptureError)
		return func() {
			if r := recover(); r != nil {
				m.CapturePanicAndWait(r, 2*time.Second)
				panic(r)
			}
		}
	case len(mode) == 0:
		// default panic behavior
		utilruntime.ReallyCrash = false
		return doNothing

	default:
		glog.Errorf("Unrecognized panic behavior")
		return doNothing
	}
}

func crashOnDelay(delay time.Duration, delayString string) func(interface{}) {
	return func(in interface{}) {
		go func() {
			glog.Errorf("Panic happened.  Process will crash in %v.", delayString)
			time.Sleep(delay)
			panic(in)
		}()
	}
}
