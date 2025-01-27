package kubeletlogparser

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func TestSyncLoopProbeParser(t *testing.T) {
	type entry struct {
		node, line string
	}

	newProbeLine := func(ts time.Time, node, probeType, status string) entry {
		const template = `%s	2546 kubelet.go:2542] "SyncLoop (probe)" probe="%s" status="%s" pod="openshift-etcd/etcd-%s"`
		return entry{
			node: node,
			line: fmt.Sprintf(template, ts.Format("Jan 02 15:04:05.000000"), probeType, status, node),
		}
	}
	newInterval := func(at time.Time, node, probeType, status string) monitorapi.Interval {
		return monitorapi.NewInterval(monitorapi.SourceKubeletLog, monitorapi.Info).
			Locator(monitorapi.NewLocator().KubeletSyncLoopProbe(node, "openshift-etcd", "etcd-"+node, probeType)).
			Message(
				monitorapi.NewMessage().
					Reason(monitorapi.IntervalReason(status)).
					Node(node).
					WithAnnotation("probe", probeType).
					WithAnnotation("status", status).
					HumanMessage("kubelet SyncLoop probe"),
			).Build(at, at)
	}

	tests := []struct {
		name  string
		setup func(t *testing.T) (entry, monitorapi.Intervals)
	}{
		{
			name: "Pod not ready, status is empty (legacy case)",
			setup: func(t *testing.T) (entry, monitorapi.Intervals) {
				at := getNow(t)
				intervals := monitorapi.Intervals{
					newInterval(at, "master-1", "readiness", "not ready"),
				}
				return newProbeLine(at, "master-1", "readiness", ""), intervals
			},
		},
		{
			name: "Pod is not ready",
			setup: func(t *testing.T) (entry, monitorapi.Intervals) {
				at := getNow(t)
				intervals := monitorapi.Intervals{
					newInterval(at, "master-1", "readiness", "not ready"),
				}
				return newProbeLine(at, "master-1", "readiness", "not ready"), intervals
			},
		},
		{
			name: "Pod is ready",
			setup: func(t *testing.T) (entry, monitorapi.Intervals) {
				at := getNow(t)
				intervals := monitorapi.Intervals{
					newInterval(at, "master-1", "readiness", "ready"),
				}
				return newProbeLine(at, "master-1", "readiness", "ready"), intervals
			},
		},
		{
			name: "Pod is live",
			setup: func(t *testing.T) (entry, monitorapi.Intervals) {
				at := getNow(t)
				intervals := monitorapi.Intervals{
					newInterval(at, "master-1", "liveness", "healthy"),
				}
				return newProbeLine(at, "master-1", "liveness", "healthy"), intervals
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry, intervalsWant := test.setup(t)
			parserFn := NewEtcdStaticPodEventsFromKubelet()

			intervalsGot := parserFn(entry.node, entry.line)
			if want, got := intervalsWant, intervalsGot; !cmp.Equal(want, got) {
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
