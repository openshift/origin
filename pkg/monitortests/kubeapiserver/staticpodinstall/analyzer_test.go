package staticpodinstall

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/staticpodinstall/kubeletlogparser"
)

func TestInstallerPodPLEGEventAnalyzer(t *testing.T) {
	type entry struct {
		node, line string
	}
	newProbeLine := func(ts time.Time, node, pod, eventType string) entry {
		const template = `%s    2546 kubelet.go:2453] "SyncLoop (PLEG): event for pod" pod="openshift-etcd/%s-%s" event={"ID":"0d817ff9-f980-46f0-b046-57ee340e2d38","Type":"%s"}`
		return entry{
			node: node,
			line: fmt.Sprintf(template, ts.Format("Jan 02 15:04:05.000000"), pod, node, eventType),
		}
	}
	newInterval := func(from, to time.Time, level monitorapi.IntervalLevel, node, msg string, annotations map[monitorapi.AnnotationKey]string) monitorapi.Interval {
		return monitorapi.NewInterval(monitorapi.SourceStaticPodInstallMonitor, level).
			Locator(monitorapi.NewLocator().StaticPodInstall(node, "installer")).
			Message(
				monitorapi.NewMessage().
					Reason("InstallerPodCompleted").
					WithAnnotations(annotations).
					HumanMessage(msg),
			).
			Display().
			Build(from, to)
	}

	tests := []struct {
		name  string
		setup func(*testing.T) ([]entry, monitorapi.Intervals)
	}{
		{
			name: "two installer pod completed on the same node",
			setup: func(t *testing.T) ([]entry, monitorapi.Intervals) {
				now := getNow(t)
				from1, to1 := now, now.Add(3*time.Second)
				from2, to2 := to1.Add(time.Second), to1.Add(4*time.Second)
				lines := []entry{
					// installer-1 pod, with [ContainerStarted, ContainerDied]
					newProbeLine(from1, "master-1", "installer-1", "ContainerStarted"),
					newProbeLine(to1, "master-1", "installer-1", "ContainerDied"),
					// installer-2 pod, with [ContainerStarted, ContainerStarted, ContainerDied, ContainerDied]
					newProbeLine(from2, "master-1", "installer-2", "ContainerStarted"),
					newProbeLine(from2.Add(time.Millisecond), "master-1", "installer-2", "ContainerStarted"),
					newProbeLine(to2.Add(-time.Millisecond), "master-1", "installer-2", "ContainerDied"),
					newProbeLine(to2, "master-1", "installer-2", "ContainerDied"),
				}
				intervals := monitorapi.Intervals{
					newInterval(from1, to1, monitorapi.Info, "master-1", fmt.Sprintf("pod=%s duration=%s", "installer-1-master-1", to1.Sub(from1)), nil),
					newInterval(from2, to2, monitorapi.Info, "master-1", fmt.Sprintf("pod=%s duration=%s", "installer-2-master-1", to2.Sub(from2)), nil),
				}

				return lines, intervals
			},
		},
		{
			name: "concurrent installer pods on two separate nodes",
			setup: func(t *testing.T) ([]entry, monitorapi.Intervals) {
				now := getNow(t)
				from1, to1 := now, now.Add(11*time.Second)
				from2, to2 := to1.Add(-time.Second), to1.Add(12*time.Second)
				lines := []entry{
					// pod=installer-1, node=master-1 events=[ContainerStarted, ContainerDied]
					newProbeLine(from1, "master-1", "installer-1", "ContainerStarted"),
					newProbeLine(to1, "master-1", "installer-1", "ContainerDied"),
					// pod=installer-1, node=master-2 events=[ContainerStarted, ContainerDied]
					newProbeLine(from2, "master-2", "installer-1", "ContainerStarted"),
					newProbeLine(to2, "master-2", "installer-1", "ContainerDied"),
				}
				msg1 := fmt.Sprintf("pod=%s duration=%s a concurrent pod started after: %s", "installer-1-master-1", to1.Sub(from1), from2.Sub(from1))
				annotations1 := map[monitorapi.AnnotationKey]string{
					"concurrent-node": "master-2",
					"concurrent-pod":  "installer-1-master-2",
				}
				intervals := monitorapi.Intervals{
					newInterval(from1, to1, monitorapi.Error, "master-1", msg1, annotations1),
					newInterval(from2, to2, monitorapi.Info, "master-2", fmt.Sprintf("pod=%s duration=%s", "installer-1-master-2", to2.Sub(from2)), nil),
				}

				return lines, intervals
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries, intervalsWant := test.setup(t)

			// step 1: create initial intervals by parsing kubelet logs
			parserFn := kubeletlogparser.NewEtcdStaticPodEventsFromKubelet()
			initial := monitorapi.Intervals{}
			for _, entry := range entries {
				initial = append(initial, parserFn(entry.node, entry.line)...)
			}

			// step 2: feed the initial intervals to the analyzers
			// and construct the computed intervals
			mt := NewStaticPodInstallMonitorTest()
			computed := mt.construct(initial)

			if want, got := intervalsWant, computed; !cmp.Equal(want, got) {
				t.Errorf("expected a match, diff: %s", cmp.Diff(want, got))
			}

		})
	}
}

func TestStaticPodReadinessProbeEventAnalyzer(t *testing.T) {
	type entry struct {
		node, line string
	}
	newProbeLine := func(ts time.Time, node, probeType, status string) entry {
		const template = `%s    2546 kubelet.go:2542] "SyncLoop (probe)" probe="%s" status="%s" pod="openshift-etcd/etcd-%s"`
		return entry{
			node: node,
			line: fmt.Sprintf(template, ts.Format("Jan 02 15:04:05.000000"), probeType, status, node),
		}
	}
	newInterval := func(from, to time.Time, level monitorapi.IntervalLevel, node, msg string, annotations map[monitorapi.AnnotationKey]string) monitorapi.Interval {
		return monitorapi.NewInterval(monitorapi.SourceStaticPodInstallMonitor, level).
			Locator(monitorapi.NewLocator().StaticPodInstall(node, "etcd")).
			Message(
				monitorapi.NewMessage().
					HumanMessage(msg).
					WithAnnotations(annotations).
					Reason(monitorapi.IntervalReason("StaticPodUnready")),
			).
			Display().
			Build(from, to)
	}

	tests := []struct {
		name  string
		setup func(*testing.T) ([]entry, monitorapi.Intervals)
	}{
		{
			name: "valid unready window, with repeating unready and ready events ",
			setup: func(t *testing.T) ([]entry, monitorapi.Intervals) {
				from := getNow(t)
				to := from.Add(11 * time.Second)
				lines := []entry{
					newProbeLine(from, "master-1", "readiness", ""),
					newProbeLine(from.Add(2*time.Second), "master-1", "readiness", "not ready"),
					newProbeLine(from.Add(4*time.Second), "master-1", "readiness", "not ready"),
					newProbeLine(from.Add(6*time.Second), "master-1", "liveness", "ready"),
					newProbeLine(to, "master-1", "readiness", "ready"),
					newProbeLine(to.Add(time.Second), "master-1", "readiness", "ready"),
				}
				intervals := monitorapi.Intervals{
					newInterval(from, to, monitorapi.Info, "master-1", fmt.Sprintf("static pod unready duration=%s", to.Sub(from)), nil),
				}

				return lines, intervals
			},
		},
		{
			name: "concurrent unready window on separate nodes",
			setup: func(t *testing.T) ([]entry, monitorapi.Intervals) {
				from1 := getNow(t)
				to1, from2, to2 := from1.Add(10*time.Second), from1.Add(time.Second), from1.Add(11*time.Second)
				lines := []entry{
					newProbeLine(from1, "master-1", "readiness", "not ready"),
					newProbeLine(to1, "master-1", "readiness", "ready"),
					newProbeLine(from2, "master-2", "readiness", "not ready"),
					newProbeLine(to2, "master-2", "readiness", "ready"),
				}
				msg1 := fmt.Sprintf("static pod unready duration=%s concurrent unready - pod: %s was unready after: %s", to1.Sub(from1), "etcd-master-2", from2.Sub(from1))
				annotations1 := map[monitorapi.AnnotationKey]string{
					"concurrent-node": "master-2",
					"concurrent-pod":  "etcd-master-2",
				}
				intervals := monitorapi.Intervals{
					newInterval(from1, to1, monitorapi.Error, "master-1", msg1, annotations1),
					newInterval(from2, to2, monitorapi.Info, "master-2", fmt.Sprintf("static pod unready duration=%s", to2.Sub(from2)), nil),
				}

				return lines, intervals
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries, intervalsWant := test.setup(t)

			// step 1: create initial intervals by parsing kubelet logs
			parserFn := kubeletlogparser.NewEtcdStaticPodEventsFromKubelet()
			initial := monitorapi.Intervals{}
			for _, entry := range entries {
				initial = append(initial, parserFn(entry.node, entry.line)...)
			}

			// step 2: feed the initial intervals to the analyzers
			// and construct the computed intervals
			mt := NewStaticPodInstallMonitorTest()
			computed := mt.construct(initial)

			if want, got := intervalsWant, computed; !cmp.Equal(want, got) {
				t.Errorf("expected a match, diff: %s", cmp.Diff(want, got))
			}
		})
	}
}

func getNow(t *testing.T) time.Time {
	// keep micro second precision, there may be a better way of doing it
	layout := "Jan 02 2006 15:04:05.000000"
	s := time.Now().Format(layout)
	ts, err := time.Parse(layout, s)
	if err != nil {
		t.Fatalf("unexpected error while getting time - %v", err)
	}
	return ts
}
