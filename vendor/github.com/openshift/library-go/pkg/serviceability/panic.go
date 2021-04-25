package serviceability

import (
	"encoding/json"
	"strings"
	"time"

	"k8s.io/klog/v2"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/version"
)

// BehaviorOnPanic is a helper for setting the crash mode of OpenShift when a panic is caught.
// It returns a function that should be the defer handler for the caller.
func BehaviorOnPanic(modeString string, productVersion version.Info) func() {
	modes := []string{}
	if err := json.Unmarshal([]byte(modeString), &modes); err != nil {
		return behaviorOnPanic(modeString, productVersion)
	}

	fns := []func(){}

	for _, mode := range modes {
		fns = append(fns, behaviorOnPanic(mode, productVersion))
	}

	return func() {
		for _, fn := range fns {
			fn()
		}
	}
}

func behaviorOnPanic(mode string, productVersion version.Info) func() {
	doNothing := func() {}

	switch {
	case mode == "crash":
		klog.Infof("Process will terminate as soon as a panic occurs.")
		utilruntime.ReallyCrash = true
		return doNothing

	case strings.HasPrefix(mode, "crash-after-delay:"):
		delayDurationString := strings.TrimPrefix(mode, "crash-after-delay:")
		delayDuration, err := time.ParseDuration(delayDurationString)
		if err != nil {
			klog.Errorf("Unable to start crash-after-delay.  Crashing immediately instead: %v", err)
			utilruntime.ReallyCrash = true
			return doNothing
		}
		klog.Infof("Process will terminate %v after a panic occurs.", delayDurationString)
		utilruntime.ReallyCrash = false
		utilruntime.PanicHandlers = append(utilruntime.PanicHandlers, crashOnDelay(delayDuration, delayDurationString))
		return doNothing

	case len(mode) == 0:
		// default panic behavior
		utilruntime.ReallyCrash = false
		return doNothing

	default:
		klog.Errorf("Unrecognized panic behavior")
		return doNothing
	}
}

func crashOnDelay(delay time.Duration, delayString string) func(interface{}) {
	return func(in interface{}) {
		go func() {
			klog.Errorf("Panic happened.  Process will crash in %v.", delayString)
			time.Sleep(delay)
			panic(in)
		}()
	}
}
