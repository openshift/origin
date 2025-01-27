package staticpodinstall

import (
	"fmt"
	"sort"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type podKey struct {
	ns, name string
}

type podWindow struct {
	podKey
	node     string
	from, to time.Time
}

func (w podWindow) isComplete() bool {
	return !w.from.IsZero() && !w.to.IsZero() && w.from.Before(w.to)
}

type staticPodReadinessProbeEventAnalyzer struct {
	// static pod unready window [from -> to]
	windowsByPod map[podKey][]*podWindow
}

func (a staticPodReadinessProbeEventAnalyzer) want(interval monitorapi.Interval) bool {
	return interval.Locator.Type == monitorapi.LocatorTypeKubeletSyncLoopProbe &&
		interval.Locator.Keys[monitorapi.LocatorTypeKubeletSyncLoopProbeType] == "readiness"
}

func (a staticPodReadinessProbeEventAnalyzer) analyze(interval monitorapi.Interval) {
	key := key(interval)
	windows, ok := a.windowsByPod[key]
	status := interval.Message.Annotations[monitorapi.AnnotationKey("status")]
	if !ok || len(windows) == 0 ||
		// we need to start a new window when we see an unready after a ready event
		(windows[len(windows)-1].isComplete() && status != "ready") {
		windows = append(windows, &podWindow{
			podKey: key,
			node:   interval.Locator.Keys[monitorapi.LocatorNodeKey],
		})
		a.windowsByPod[key] = windows
	}
	window := windows[len(windows)-1]

	// NOTE: if a ready event goes missing:
	//  unready unready ready(missing) unready ready
	// these two unready windows will be reported as one
	switch status {
	case "ready":
		if window.isComplete() {
			// the last event we saw was a ready
			// event, so we can ignore this one
			break
		}
		// we will take the earliest ready event time, since there could be
		// multiple ready events reported in sequence
		if window.to.IsZero() {
			window.to = interval.From
		}
	default:
		// we will retain the earliest unready time, since there usually are
		// multiple unready events before kubelet reports a ready event
		if window.from.IsZero() {
			window.from = interval.From
		}
	}
}

func (a staticPodReadinessProbeEventAnalyzer) result() monitorapi.Intervals {
	sorted := sortedByFrom{}
	for _, v := range a.windowsByPod {
		sorted = append(sorted, v...)
	}
	sort.Sort(sorted)

	intervals := monitorapi.Intervals{}
	for i, this := range sorted {
		level := monitorapi.Error
		annotations := map[monitorapi.AnnotationKey]string{}
		msg := fmt.Sprintf("static pod unready duration=%s", this.to.Sub(this.from))
		if this.isComplete() {
			level = monitorapi.Info
			for _, other := range sorted[i+1:] {
				if !other.isComplete() {
					continue
				}
				if other.from.After(this.to) {
					break
				}
				// a) we are on a different node
				// b) is there any installer pod that is active?
				if this.node != other.node {
					level = monitorapi.Error
					annotations["concurrent-node"] = other.node
					annotations["concurrent-pod"] = other.name
					msg = fmt.Sprintf("%s concurrent unready - pod: %s was unready after: %s", msg, other.name, other.from.Sub(this.from))
					break
				}
			}
		}

		interval := monitorapi.NewInterval(monitorapi.SourceStaticPodInstallMonitor, level).
			Locator(monitorapi.NewLocator().StaticPodInstall(this.node, "etcd")).
			Message(monitorapi.NewMessage().
				HumanMessage(msg).
				WithAnnotations(annotations).
				Reason(monitorapi.IntervalReason("StaticPodUnready")),
			).
			Display().
			Build(this.from, this.to)
		intervals = append(intervals, interval)
	}

	return intervals

}

type installerPodPLEGEventAnalyzer struct {
	// installer pods run window [from -> to]
	windows map[podKey]*podWindow
}

func (a installerPodPLEGEventAnalyzer) want(interval monitorapi.Interval) bool {
	return interval.Locator.Type == monitorapi.LocatorTypeKubeletSyncLoopPLEG
}

func (a installerPodPLEGEventAnalyzer) analyze(interval monitorapi.Interval) {
	key := key(interval)
	window, ok := a.windows[key]
	if !ok {
		window = &podWindow{
			podKey: key,
			node:   interval.Locator.Keys[monitorapi.LocatorNodeKey],
		}
		a.windows[key] = window
	}
	event := interval.Locator.Keys[monitorapi.LocatorTypeKubeletSyncLoopPLEGType]
	switch event {
	case "ContainerStarted":
		// we will take the earliest start time (due to multiple containers)
		// TODO: container name is not provided in these events by kubelet
		if window.from.IsZero() {
			window.from = interval.From
		}
	case "ContainerDied":
		// we will take the most recent died time (due to multiple containers)
		// TODO: container name is not provided in these events by kubelet
		window.to = interval.From
	}
}

func (a installerPodPLEGEventAnalyzer) result() monitorapi.Intervals {
	sorted := sortedByFrom{}
	for _, v := range a.windows {
		sorted = append(sorted, v)
	}
	sort.Sort(sorted)

	intervals := monitorapi.Intervals{}
	for i, this := range sorted {
		level := monitorapi.Error
		annotations := map[monitorapi.AnnotationKey]string{}
		msg := fmt.Sprintf("pod=%s duration=%s", this.name, this.to.Sub(this.from))
		if this.isComplete() {
			level = monitorapi.Info
			for _, other := range sorted[i+1:] {
				if !other.isComplete() {
					continue
				}
				if other.from.After(this.to) {
					break
				}
				// a) we are on a different node
				// b) is there any installer pod that is active?
				if this.node != other.node {
					level = monitorapi.Error
					annotations["concurrent-node"] = other.node
					annotations["concurrent-pod"] = other.name
					msg = fmt.Sprintf("%s a concurrent pod started after: %s", msg, other.from.Sub(this.from))
					break
				}
			}
		}

		interval := monitorapi.NewInterval(monitorapi.SourceStaticPodInstallMonitor, level).
			Locator(monitorapi.NewLocator().StaticPodInstall(this.node, "installer")).
			Message(monitorapi.NewMessage().
				HumanMessage(msg).
				WithAnnotations(annotations).
				Reason(monitorapi.IntervalReason("InstallerPodCompleted")),
			).
			Display().
			Build(this.from, this.to)
		intervals = append(intervals, interval)
	}

	return intervals
}

func key(interval monitorapi.Interval) podKey {
	return podKey{
		ns:   interval.Locator.Keys[monitorapi.LocatorNamespaceKey],
		name: interval.Locator.Keys[monitorapi.LocatorPodKey],
	}
}

type sortedByFrom []*podWindow

func (s sortedByFrom) Len() int           { return len(s) }
func (s sortedByFrom) Less(i, j int) bool { return s[i].from.Before(s[j].from) }
func (s sortedByFrom) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
