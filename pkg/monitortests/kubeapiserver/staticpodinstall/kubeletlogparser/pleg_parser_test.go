package kubeletlogparser

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func TestSyncLoopPLEGParser(t *testing.T) {
	type entry struct {
		node, line string
	}

	newProbeLine := func(ts time.Time, node, pod, eventType string) entry {
		const template = `%s 2546 kubelet.go:2453] "SyncLoop (PLEG): event for pod" pod="openshift-etcd/%s-%s" event={"ID":"0d817ff9-f980-46f0-b046-57ee340e2d38","Type":"%s","Data":"f8d11fe0b65575141b38a7310faebaff0b287779bc27d3c635a144891a2304fa"}`
		return entry{
			node: node,
			line: fmt.Sprintf(template, ts.Format("Jan 02 15:04:05.000000"), pod, node, eventType),
		}
	}
	newInterval := func(at time.Time, node, pod, eventType string) monitorapi.Interval {
		return monitorapi.NewInterval(monitorapi.SourceKubeletLog, monitorapi.Info).
			Locator(monitorapi.NewLocator().KubeletSyncLoopPLEG(node, "openshift-etcd", pod+"-"+node, eventType)).
			Message(
				monitorapi.NewMessage().
					Reason(monitorapi.IntervalReason(eventType)).
					Node(node).
					WithAnnotation("type", eventType).
					HumanMessage("kubelet PLEG event"),
			).Build(at, at)

	}

	tests := []struct {
		name  string
		setup func(t *testing.T) (entry, monitorapi.Intervals)
	}{
		{

			name: "PLEG ContainerStarted event",
			setup: func(t *testing.T) (entry, monitorapi.Intervals) {
				at := getNow(t)
				intervals := monitorapi.Intervals{
					newInterval(at, "master-1", "installer-1", "ContainerStarted"),
				}
				return newProbeLine(at, "master-1", "installer-1", "ContainerStarted"), intervals
			},
		},
		{

			name: "PLEG ContainerDied event",
			setup: func(t *testing.T) (entry, monitorapi.Intervals) {
				at := getNow(t)
				intervals := monitorapi.Intervals{
					newInterval(at, "master-1", "installer-1", "ContainerDied"),
				}
				return newProbeLine(at, "master-1", "installer-1", "ContainerDied"), intervals
			},
		},
		{

			name: "unwanted PLEG event, should be ignored",
			setup: func(t *testing.T) (entry, monitorapi.Intervals) {
				at := getNow(t)
				return newProbeLine(at, "master-1", "foo-1", "ContainerDied"), monitorapi.Intervals{}
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
