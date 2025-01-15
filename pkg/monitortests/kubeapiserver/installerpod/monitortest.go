package installerpod

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	MonitorName = "installer-pod-monitor"
)

func NewInstallerPodMonitorTest() monitortestframework.MonitorTest {
	return &monitorTest{
		monitor: &installerPodMonitor{
			pods: map[string]*podInfo{},
		},
		filter: func(interval monitorapi.Interval) bool {
			if ns, ok := interval.Locator.Keys[monitorapi.LocatorNamespaceKey]; !ok || ns != "openshift-etcd" {
				return false
			}
			if name, ok := interval.Locator.Keys[monitorapi.LocatorKey("pod")]; !ok || !strings.HasPrefix(name, "installer-") {
				return false
			}

			switch interval.Message.Reason {
			case "Created", "Started", "Killing", "StaticPodInstallerCompleted":
				return true
			default:
				return false
			}
		},
	}
}

type monitorTest struct {
	monitor *installerPodMonitor
	filter  func(interval monitorapi.Interval) bool
}

func (mt *monitorTest) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (mt *monitorTest) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (mt *monitorTest) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	computed := mt.monitor.process(startingIntervals, mt.filter)
	return computed, nil
}

func (mt *monitorTest) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	junitTest := &junitTest{
		name:           "[sig-apimachinery] installer Pods should not run concurrently on two or more node",
		concurrentPods: mt.monitor.concurrentPods,
	}

	framework.Logf("monitor[%s]: found %d occurrences of installer pods running concurrently on two or more nodes", MonitorName, len(junitTest.concurrentPods))

	// the following constraints define pass/fail for this test:
	// a) if we don't find any installer pod activity, then
	// this test is a noop, so we mark the test as skipped
	// b) we find installer pod activity, but no two nodes are running
	// these pods concurrently, this test is a pass
	// c) we find installer pod activity, and at least one incident of two
	// or more nodes running these pods concurrently, this test is a flake/fail
	if len(mt.monitor.interested) == 0 {
		// a) no installer pod activity observed, mark the test as skipped
		return junitTest.Skip(), nil
	}
	return junitTest.Result(), nil // b or c
}

func (*monitorTest) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*monitorTest) WantSignificantlyOldEvents() bool { return true }

func (*monitorTest) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

type podInfo struct {
	node               string
	name               string
	namespace          string
	lastReason         string
	reasons            []string
	startedAt, endedAt time.Time
	concurrent         bool
	old                bool
}

func (pi *podInfo) String() string {
	return fmt.Sprintf("node(%s) name(%s) namespace(%s) reason(%v) old(%t) started(%s) duration: %s",
		pi.node, pi.name, pi.namespace, strings.Join(pi.reasons, ","), pi.old,
		pi.startedAt.Format(time.RFC3339), pi.endedAt.Sub(pi.startedAt))
}

// placehoder for two concurrent installer Pods
type concurrentPods struct {
	this, that *podInfo
}

type installerPodMonitor struct {
	// interested events after filter is applied
	interested monitorapi.Intervals
	// as each inyerested event is processed, we track states here
	pods map[string]*podInfo
	// occurrences of concurrent Pods are tracked here
	concurrentPods []concurrentPods
}

func (m *installerPodMonitor) process(intervals monitorapi.Intervals, filter func(interval monitorapi.Interval) bool) monitorapi.Intervals {
	m.interested = make(monitorapi.Intervals, 0)
	for _, interval := range intervals {
		if filter(interval) {
			m.interested = append(m.interested, interval)
		}
	}

	framework.Logf("monitor[%s]: processing %d events", MonitorName, len(m.interested))
	for _, interval := range m.interested {
		m.processOne(interval)
	}

	computed := monitorapi.Intervals{}
	for podName, info := range m.pods {
		level := monitorapi.Info
		endedAt := info.endedAt
		if endedAt.IsZero() {
			endedAt = info.startedAt
			level = monitorapi.Error
		}
		if info.lastReason == "Killing" || info.concurrent {
			level = monitorapi.Error
		}

		concurrentMsg := ""
		if info.concurrent {
			concurrentMsg = fmt.Sprintf("installer Pods may be running concurrently on at least two nodes")
		}
		computed = append(computed,
			monitorapi.NewInterval(monitorapi.SourceInstallerPodMonitor, level).
				Locator(monitorapi.NewLocator().NodeFromName(info.node)).
				Message(monitorapi.NewMessage().
					HumanMessage(fmt.Sprintf("%s %s", podName, concurrentMsg)).
					Reason(monitorapi.IntervalReason(info.lastReason)),
				).
				Display().
				Build(info.startedAt, endedAt),
		)
	}

	return computed
}

func (m *installerPodMonitor) processOne(interval monitorapi.Interval) {
	hostname := host(interval)
	if len(hostname) == 0 {
		framework.Logf("monitor[%s]: no host name for interval: %+v", MonitorName, interval)
		return
	}

	if interval.SignificantlyBefore {
		framework.Logf("monitor[%s]: seeing an old event: %+v", MonitorName, interval)
	}

	thisPodName := interval.Locator.Keys[monitorapi.LocatorKey("pod")]
	reason := string(interval.Message.Reason)
	switch reason {
	case "Started":
		if _, ok := m.pods[thisPodName]; ok {
			framework.Logf("monitor[%s]: unexpected, seeing Started twice for the same event: %+v", MonitorName, interval)
			return
		}
		thisPod := &podInfo{
			node:      hostname,
			name:      thisPodName,
			namespace: interval.Locator.Keys[monitorapi.LocatorNamespaceKey],
			startedAt: interval.From,
			reasons:   []string{reason},
			old:       interval.SignificantlyBefore,
		}
		m.pods[thisPodName] = thisPod
		// TODO: are any other installer pods active on a different node?
		for _, otherPod := range m.pods {
			if otherPod.node == thisPod.node {
				continue
			}
			// a) we are on a different node
			// b) is there any installer pod that is active?
			if !otherPod.startedAt.IsZero() && otherPod.endedAt.IsZero() {
				thisPod.concurrent, otherPod.concurrent = true, true
				m.concurrentPods = append(m.concurrentPods, concurrentPods{this: otherPod, that: thisPod})
			}
		}

	// these events denote the end of an installer pod, in my investigation of
	// a failed run, i see one or the other, never both for an installer pod.
	case "Killing", "StaticPodInstallerCompleted":
		info, ok := m.pods[thisPodName]
		if !ok {
			framework.Logf("monitor[%s]: unexpected, not seen Started before - event: %+v", MonitorName, interval)
			return
		}
		info.lastReason = reason
		info.reasons = append(info.reasons, reason)
		info.endedAt = interval.From
	}
}

func host(interval monitorapi.Interval) string {
	// kubelet events
	if host, ok := interval.Locator.Keys[monitorapi.LocatorNodeKey]; ok && len(host) > 0 {
		return host
	}

	// StaticPodInstallerCompleted is reported by the installer pod, and it
	// does not contain any source information
	name := interval.Locator.Keys[monitorapi.LocatorKey("pod")]
	if len(name) == 0 {
		return ""
	}

	// installer pod name has the following format:
	//   - installer-5-retry-1-ci-op-cn7ykf7p-b9a0c-bxxcm-master-2
	//   - installer-7-ci-op-cn7ykf7p-b9a0c-bxxcm-master-2
	_, after, found := strings.Cut(name, "-retry-")
	if found {
		if split := strings.SplitN(after, "-", 2); len(split) == 2 {
			return split[1]
		}
		return ""
	}
	if split := strings.SplitN(name, "-", 3); len(split) == 3 {
		return split[2]
	}
	return ""
}

type junitTest struct {
	name           string
	concurrentPods []concurrentPods
}

func (jut *junitTest) Skip() []*junitapi.JUnitTestCase {
	skipped := &junitapi.JUnitTestCase{
		Name: jut.name,
		SkipMessage: &junitapi.SkipMessage{
			Message: "No installer pod activity found",
		},
	}
	return []*junitapi.JUnitTestCase{skipped}
}

func (jut *junitTest) Result() []*junitapi.JUnitTestCase {
	passed := &junitapi.JUnitTestCase{
		Name:      jut.name,
		SystemOut: "",
	}
	if len(jut.concurrentPods) == 0 {
		// passed
		return []*junitapi.JUnitTestCase{passed}
	}

	failed := &junitapi.JUnitTestCase{
		Name:          jut.name,
		SystemOut:     fmt.Sprintf("installer pods running concurrently on two or more nodes"),
		FailureOutput: &junitapi.FailureOutput{},
	}
	for _, concurrent := range jut.concurrentPods {
		a, b := concurrent.this, concurrent.that
		msg := fmt.Sprintf("A(%s -> %s) B(%s -> %s):\nA: %s\nB: %s\n", a.startedAt.Format(time.RFC3339),
			a.endedAt.Format(time.RFC3339), b.startedAt.Format(time.RFC3339), b.endedAt.Format(time.RFC3339), a, b)
		failed.FailureOutput.Output = fmt.Sprintf("%s\n%s", failed.FailureOutput.Output, msg)
	}

	// TODO: for now, we flake the test, Once we know it's fully
	// passing then we can remove the flake test case.
	return []*junitapi.JUnitTestCase{failed, passed}
}
