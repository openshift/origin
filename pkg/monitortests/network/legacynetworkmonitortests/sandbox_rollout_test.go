package legacynetworkmonitortests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"k8s.io/client-go/rest"
)

const missingCNIMessage = "Failed to create pod sandbox: no CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?"

func sandboxTestConfig(t *testing.T) *rest.Config {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/namespaces/kube-public/configmaps/microshift-version":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"NotFound","code":404}`))
		case "/apis/config.openshift.io/v1/infrastructures/cluster":
			_, _ = w.Write([]byte(`{"kind":"Infrastructure","apiVersion":"config.openshift.io/v1","status":{"platformStatus":{"type":"AWS"}}}`))
		default:
			t.Errorf("unexpected API request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return &rest.Config{Host: server.URL}
}

func sandboxRolloutEvents() monitorapi.Intervals {
	start := time.Date(2026, 11, 11, 18, 50, 0, 0, time.UTC)
	at := func(seconds int) time.Time { return start.Add(time.Duration(seconds) * time.Second) }
	event := func(source monitorapi.IntervalSource, kind monitorapi.LocatorType, reason monitorapi.IntervalReason, from, to int, keys map[monitorapi.LocatorKey]string) monitorapi.Interval {
		return monitorapi.Interval{
			Source: source, From: at(from), To: at(to),
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{Type: kind, Keys: keys},
				Message: monitorapi.Message{Reason: reason, Annotations: map[monitorapi.AnnotationKey]string{}},
			},
		}
	}
	operator := event(monitorapi.SourceOperatorState, monitorapi.LocatorTypeClusterOperator, "Deploying", -94, 2,
		map[monitorapi.LocatorKey]string{monitorapi.LocatorClusterOperatorKey: "network"})
	operator.Message.Annotations[monitorapi.AnnotationCondition] = "Progressing"
	operator.Message.Annotations[monitorapi.AnnotationStatus] = "True"
	deletion := event(monitorapi.SourcePodMonitor, monitorapi.LocatorTypePod, monitorapi.PodReasonGracefulDeleteStarted, 0, 0,
		map[monitorapi.LocatorKey]string{"namespace": "openshift-ovn-kubernetes", "pod": "ovnkube-node-old", "node": "node-1", "uid": "ovn-old"})
	scheduled := event(monitorapi.SourcePodState, monitorapi.LocatorTypePod, monitorapi.PodReasonScheduled, 2, 100,
		map[monitorapi.LocatorKey]string{"namespace": "workload", "pod": "target", "uid": "target-uid"})
	scheduled.Message.Annotations[monitorapi.AnnotationNode] = "node-1"
	failure := event(monitorapi.SourceKubeEvent, monitorapi.LocatorTypeKind, "FailedCreatePodSandBox", 7, 8,
		map[monitorapi.LocatorKey]string{"namespace": "workload", "pod": "target", "node": "node-1"})
	failure.Message.HumanMessage = missingCNIMessage
	failure.Message.Annotations["firstTimestamp"] = at(7).Format(time.RFC3339)
	failure.Message.Annotations["lastTimestamp"] = at(7).Format(time.RFC3339)
	failure.Message.Annotations[monitorapi.AnnotationEventUID] = "failure-event-not-pod-uid"
	recovery := event(monitorapi.SourceKubeEvent, monitorapi.LocatorTypeKind, "AddedInterface", 22, 22,
		map[monitorapi.LocatorKey]string{"namespace": "workload", "pod": "target"})
	recovery.Message.HumanMessage = "Add eth0 [10.128.2.3/23] from ovn-kubernetes"
	recovery.Message.Annotations["firstTimestamp"] = at(22).Format(time.RFC3339)
	recovery.Message.Annotations["lastTimestamp"] = at(22).Format(time.RFC3339)
	return monitorapi.Intervals{operator, deletion, scheduled, failure, recovery}
}

func TestSandboxCreationDuringOVNRollout(t *testing.T) {
	config := sandboxTestConfig(t)
	tests := []struct {
		name   string
		change func(monitorapi.Intervals) monitorapi.Intervals
		flake  bool
	}{
		{name: "recovered after network stopped progressing", flake: true},
		{name: "unsorted intervals", flake: true, change: func(e monitorapi.Intervals) monitorapi.Intervals { slices.Reverse(e); return e }},
		{name: "missing recovery", change: func(e monitorapi.Intervals) monitorapi.Intervals { return e[:4] }},
		{name: "other node", change: func(e monitorapi.Intervals) monitorapi.Intervals { e[1].Locator.Keys["node"] = "node-2"; return e }},
		{name: "empty node", change: func(e monitorapi.Intervals) monitorapi.Intervals { delete(e[3].Locator.Keys, "node"); return e }},
		{name: "wrong OVN namespace", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[1].Locator.Keys["namespace"] = "workload"
			return e
		}},
		{name: "unrelated deleted pod", change: func(e monitorapi.Intervals) monitorapi.Intervals { e[1].Locator.Keys["pod"] = "other"; return e }},
		{name: "not progressing", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[0].Message.Annotations[monitorapi.AnnotationStatus] = "False"
			return e
		}},
		{name: "other operator", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[0].Locator.Keys["clusteroperator"] = "dns"
			return e
		}},
		{name: "deletion outside progressing", change: func(e monitorapi.Intervals) monitorapi.Intervals { e[0].To = e[1].From.Add(-time.Second); return e }},
		{name: "deletion at progressing boundary", flake: true, change: func(e monitorapi.Intervals) monitorapi.Intervals { e[0].To = e[1].From; return e }},
		{name: "error before deletion", change: func(e monitorapi.Intervals) monitorapi.Intervals { e[1].From = e[3].From.Add(time.Second); return e }},
		{name: "error tied with deletion", change: func(e monitorapi.Intervals) monitorapi.Intervals { e[1].From = e[3].From; return e }},
		{name: "secondary interface", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[4].Message.HumanMessage = "Add net1 [10.0.0.1/24] from another-network"
			return e
		}},
		{name: "another pod recovered", change: func(e monitorapi.Intervals) monitorapi.Intervals { e[4].Locator.Keys["pod"] = "another"; return e }},
		{name: "recovery on another node", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[4].Locator.Keys["node"] = "node-2"
			return e
		}},
		{name: "another namespace recovered", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[4].Locator.Keys["namespace"] = "another"
			return e
		}},
		{name: "recovery before last error", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[3].Message.Annotations["lastTimestamp"] = e[4].From.Add(time.Second).Format(time.RFC3339)
			e[3].From = e[4].From.Add(time.Second)
			e[3].To = e[3].From.Add(time.Second)
			return e
		}},
		{name: "recovery tied with last error", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[3].Message.Annotations["lastTimestamp"] = e[4].From.Format(time.RFC3339)
			e[3].From = e[4].From
			e[3].To = e[3].From.Add(time.Second)
			return e
		}},
		{name: "missing first occurrence", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			delete(e[3].Message.Annotations, "firstTimestamp")
			return e
		}},
		{name: "malformed last occurrence", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[3].Message.Annotations["lastTimestamp"] = "invalid"
			return e
		}},
		{name: "aggregate began before rollout", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[3].Message.Annotations["firstTimestamp"] = e[1].From.Add(-time.Minute).Format(time.RFC3339)
			return e
		}},
		{name: "aggregate recovery began before error", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[4].Message.Annotations["firstTimestamp"] = e[3].From.Add(-time.Second).Format(time.RFC3339)
			return e
		}},
		{name: "recovery beyond grace", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[4].From = e[1].From.Add(61 * time.Second)
			e[4].To = e[4].From
			e[4].Message.Annotations["firstTimestamp"] = e[4].From.Format(time.RFC3339)
			e[4].Message.Annotations["lastTimestamp"] = e[4].From.Format(time.RFC3339)
			return e
		}},
		{name: "missing lifecycle", change: func(e monitorapi.Intervals) monitorapi.Intervals { return append(e[:2], e[3:]...) }},
		{name: "missing pod UID", change: func(e monitorapi.Intervals) monitorapi.Intervals { delete(e[2].Locator.Keys, "uid"); return e }},
		{name: "ambiguous raw pod lifecycle", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			other := e[2]
			other.Source = monitorapi.SourcePodMonitor
			other.Locator.Keys = map[monitorapi.LocatorKey]string{"namespace": "workload", "pod": "target", "node": "node-1"}
			return append(e, other)
		}},
		{name: "UID-less scheduled state on another node", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			other := e[2]
			other.Locator.Keys = map[monitorapi.LocatorKey]string{"namespace": "workload", "pod": "target"}
			other.Message.Annotations = map[monitorapi.AnnotationKey]string{monitorapi.AnnotationNode: "node-2"}
			other.From = e[4].From.Add(time.Second)
			other.To = other.From.Add(time.Minute)
			return append(e, other)
		}},
		{name: "recovery at grace boundary", flake: true, change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[4].From = e[1].From.Add(time.Minute)
			e[4].To = e[4].From
			e[4].Message.Annotations["firstTimestamp"] = e[4].From.Format(time.RFC3339)
			e[4].Message.Annotations["lastTimestamp"] = e[4].From.Format(time.RFC3339)
			return e
		}},
		{name: "target deleted before recovery", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			other := e[2]
			other.Source = monitorapi.SourcePodMonitor
			other.Message.Reason = monitorapi.PodReasonGracefulDeleteStarted
			other.From = e[4].From.Add(-time.Second)
			other.To = other.From
			return append(e, other)
		}},
		{name: "lifecycle on other node", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[2].Message.Annotations[monitorapi.AnnotationNode] = "node-2"
			return e
		}},
		{name: "lifecycle does not cover recovery", change: func(e monitorapi.Intervals) monitorapi.Intervals { e[2].To = e[4].From.Add(-time.Second); return e }},
		{name: "pod recreated", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			other := e[2]
			other.Locator.Keys = map[monitorapi.LocatorKey]string{"namespace": "workload", "pod": "target", "uid": "replacement"}
			return append(e, other)
		}},
		{name: "another OVN deletion before recovery", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			other := e[1]
			other.From = other.From.Add(10 * time.Second)
			other.To = other.From
			return append(e, other)
		}},
		{name: "different error", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			e[3].Message.HumanMessage = "Failed to create pod sandbox: persistent CNI error"
			return e
		}},
		{name: "hard failure dominates recovered flake", change: func(e monitorapi.Intervals) monitorapi.Intervals {
			other := e[3]
			other.Message.HumanMessage = "Failed to create pod sandbox: persistent CNI error"
			return append(e, other)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := sandboxRolloutEvents()
			if tt.change != nil {
				events = tt.change(events)
			}
			assertSandboxOtherResult(t, events, config, tt.flake)
		})
	}
}

func assertSandboxOtherResult(t *testing.T, events monitorapi.Intervals, config *rest.Config, wantFlake bool) {
	t.Helper()
	var failed, passed int
	var output string
	for _, result := range testPodSandboxCreation(events, config, "") {
		if !strings.HasSuffix(result.Name, " by other") {
			continue
		}
		if result.FailureOutput != nil {
			failed++
			output += result.FailureOutput.Output
		} else if result.SkipMessage == nil {
			passed++
		}
	}
	wantPassed := 0
	if wantFlake {
		wantPassed = 1
	}
	if failed != 1 || passed != wantPassed {
		t.Fatalf("got %d failures and %d passes, want 1 failure and %d passes; output:\n%s", failed, passed, wantPassed, output)
	}
	if wantFlake && !strings.Contains(output, "OVN rollout") {
		t.Fatalf("missing rollout evidence in flake output: %s", output)
	}
}

func TestSandboxCreationRolloutReplay(t *testing.T) {
	data, err := os.ReadFile("testdata/sandbox-rollouts.json")
	if err != nil {
		t.Fatal(err)
	}
	var runs []struct {
		BuildID string          `json:"build_id"`
		Items   json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(data, &runs); err != nil {
		t.Fatal(err)
	}
	if len(runs) != 5 {
		t.Fatalf("expected the five reported runs, got %d", len(runs))
	}
	config := sandboxTestConfig(t)
	for _, run := range runs {
		t.Run(run.BuildID, func(t *testing.T) {
			events, err := monitorserialization.IntervalsFromJSON(append(append([]byte(`{"items":`), run.Items...), '}'))
			if err != nil {
				t.Fatal(err)
			}
			assertSandboxOtherResult(t, events, config, true)
			var withoutRecovery monitorapi.Intervals
			for _, event := range events {
				if event.Message.Reason != "AddedInterface" {
					withoutRecovery = append(withoutRecovery, event)
				}
			}
			assertSandboxOtherResult(t, withoutRecovery, config, false)
		})
	}
}
