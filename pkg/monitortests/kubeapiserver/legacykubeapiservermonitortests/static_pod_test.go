package legacykubeapiservermonitortests

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func Test_staticPodFailureFromMessage(t *testing.T) {
	type args struct {
		message string
	}
	tests := []struct {
		name    string
		args    args
		want    *staticPodFailure
		wantErr bool
	}{
		{
			name: "parser",
			args: args{message: `static pod lifecycle failure - static pod: "etcd" in namespace: "openshift-etcd" for revision: 6 on node: "ovirt10-gh8t5-master-2" didn't show up, waited: 2m30s`},
			want: &staticPodFailure{
				podName:        "etcd",
				podNamespace:   "openshift-etcd",
				node:           "ovirt10-gh8t5-master-2",
				revision:       6,
				failureMessage: `static pod lifecycle failure - static pod: "etcd" in namespace: "openshift-etcd" for revision: 6 on node: "ovirt10-gh8t5-master-2" didn't show up, waited: 2m30s`,
			},
		},
		{
			name: "I hate regex: metal string",
			args: args{
				message: `static pod lifecycle failure - static pod: "kube-controller-manager" in namespace: "openshift-kube-controller-manager" for revision: 30 on node: "master-1.ostest.test.metalkube.org" didn't show up, waited: 3m0s`,
			},
			want: &staticPodFailure{
				podName:        "kube-controller-manager",
				podNamespace:   "openshift-kube-controller-manager",
				node:           "master-1.ostest.test.metalkube.org",
				revision:       30,
				failureMessage: `static pod lifecycle failure - static pod: "kube-controller-manager" in namespace: "openshift-kube-controller-manager" for revision: 30 on node: "master-1.ostest.test.metalkube.org" didn't show up, waited: 3m0s`,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := staticPodFailureFromMessage(tt.args.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("staticPodFailureFromMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("staticPodFailureFromMessage() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStaticPodFailureRecovered(t *testing.T) {
	t.Parallel()
	failureTime := time.Date(2026, 11, 1, 8, 21, 27, 0, time.UTC)
	readyEvent := func(node string, revision int, observed time.Time) eventsv1.Event {
		return eventsv1.Event{
			Reason:    "NodeCurrentRevisionChanged",
			Note:      fmt.Sprintf("Updated node %q from revision 13 to %d because static pod is ready", node, revision),
			EventTime: metav1.NewMicroTime(observed),
		}
	}
	tests := []struct {
		name      string
		revision  int64
		events    []eventsv1.Event
		mutatePod func(*corev1.Pod)
		noPod     bool
		noTime    bool
		apiError  bool
		repeated  bool
		want      bool
	}{
		{name: "green history earlier revision 21 readiness cannot forgive missing revision 21", revision: 21, events: []eventsv1.Event{readyEvent("node", 21, failureTime.Add(-6*time.Minute-32*time.Second))}, noPod: true},
		{name: "red history earlier revision 15 cannot forgive missing revision 19", revision: 19, events: []eventsv1.Event{readyEvent("node", 15, failureTime.Add(-16*time.Minute-15*time.Second))}, noPod: true},
		{name: "red history restored revision 19 mirror pod", revision: 19, events: []eventsv1.Event{readyEvent("node", 15, failureTime.Add(-16*time.Minute-15*time.Second))}, want: true},
		{name: "green history restored revision 21 mirror pod", revision: 21, events: []eventsv1.Event{readyEvent("node", 21, failureTime.Add(-6*time.Minute-32*time.Second))}, want: true},
		{name: "same revision recovery without revision event", want: true},
		{name: "later revision mirror pod", mutatePod: func(pod *corev1.Pod) { pod.Labels["revision"] = "22" }, want: true},
		{name: "later same revision event", events: []eventsv1.Event{readyEvent("node", 19, failureTime.Add(time.Second))}, noPod: true, want: true},
		{name: "later higher revision event", events: []eventsv1.Event{readyEvent("node", 20, failureTime.Add(time.Second))}, noPod: true, want: true},
		{name: "later lower revision event", events: []eventsv1.Event{readyEvent("node", 18, failureTime.Add(time.Second))}, noPod: true},
		{name: "different node event including node substring", events: []eventsv1.Event{readyEvent("node-other", 19, failureTime.Add(time.Second))}, noPod: true},
		{name: "equal timestamp event", events: []eventsv1.Event{readyEvent("node", 19, failureTime)}, noPod: true},
		{name: "event without timestamp", events: []eventsv1.Event{readyEvent("node", 19, time.Time{})}, noPod: true},
		{name: "unknown failure timestamp", noTime: true},
		{name: "absent recovery", noPod: true},
		{name: "readiness between first and repeated failure", events: []eventsv1.Event{readyEvent("node", 19, failureTime.Add(time.Second))}, repeated: true},
		{name: "unrelated event reason", events: []eventsv1.Event{{Reason: "Other", Note: readyEvent("node", 19, failureTime.Add(time.Second)).Note, EventTime: metav1.NewMicroTime(failureTime.Add(time.Second))}}, noPod: true},
		{name: "malformed event", events: []eventsv1.Event{{Reason: "NodeCurrentRevisionChanged", Note: "unparseable", EventTime: metav1.NewMicroTime(failureTime.Add(time.Second))}}, noPod: true},
		{name: "overflowing revision event", events: []eventsv1.Event{{Reason: "NodeCurrentRevisionChanged", Note: `Updated node "node" from revision 13 to 999999999999999999999999 because static pod is ready`, EventTime: metav1.NewMicroTime(failureTime.Add(time.Second))}}, noPod: true},
		{name: "stale pod readiness", mutatePod: func(pod *corev1.Pod) {
			pod.Status.Conditions[0].LastTransitionTime = metav1.NewTime(failureTime.Add(-time.Second))
		}},
		{name: "equal pod readiness timestamp", mutatePod: func(pod *corev1.Pod) { pod.Status.Conditions[0].LastTransitionTime = metav1.NewTime(failureTime) }},
		{name: "unknown pod readiness timestamp", mutatePod: func(pod *corev1.Pod) { pod.Status.Conditions[0].LastTransitionTime = metav1.Time{} }},
		{name: "pod not ready", mutatePod: func(pod *corev1.Pod) { pod.Status.Conditions[0].Status = corev1.ConditionFalse }},
		{name: "pod readiness absent", mutatePod: func(pod *corev1.Pod) { pod.Status.Conditions = nil }},
		{name: "pod on different node", mutatePod: func(pod *corev1.Pod) { pod.Spec.NodeName = "other-node" }},
		{name: "pod in different namespace", mutatePod: func(pod *corev1.Pod) { pod.Namespace = "other-namespace" }},
		{name: "different pod", mutatePod: func(pod *corev1.Pod) { pod.Name = "other-node" }},
		{name: "non mirror pod", mutatePod: func(pod *corev1.Pod) { pod.Annotations = nil }},
		{name: "deleting pod", mutatePod: func(pod *corev1.Pod) {
			timestamp := metav1.NewTime(failureTime.Add(time.Second))
			pod.DeletionTimestamp = &timestamp
		}},
		{name: "pod not running", mutatePod: func(pod *corev1.Pod) { pod.Status.Phase = corev1.PodFailed }},
		{name: "older pod revision", mutatePod: func(pod *corev1.Pod) { pod.Labels["revision"] = "18" }},
		{name: "invalid pod revision", mutatePod: func(pod *corev1.Pod) { pod.Labels["revision"] = "invalid" }},
		{name: "pod lookup failure", apiError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			revision := test.revision
			if revision == 0 {
				revision = 19
			}
			failure := staticPodFailure{podName: "etcd", podNamespace: "openshift-etcd", node: "node", revision: revision, lastObserved: failureTime}
			if test.repeated {
				failure.lastObserved = staticPodEventTime(eventsv1.Event{EventTime: metav1.NewMicroTime(failureTime), Series: &eventsv1.EventSeries{LastObservedTime: metav1.NewMicroTime(failureTime.Add(2 * time.Second))}})
			}
			if test.noTime {
				failure.lastObserved = time.Time{}
			}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "etcd-node", Namespace: "openshift-etcd", Labels: map[string]string{"revision": fmt.Sprint(revision)}, Annotations: map[string]string{"kubernetes.io/config.mirror": "hash"}},
				Spec:       corev1.PodSpec{NodeName: "node"},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(failureTime.Add(time.Second))}}},
			}
			if test.mutatePod != nil {
				test.mutatePod(pod)
			}
			var objects []runtime.Object
			if !test.noPod {
				objects = append(objects, pod)
			}
			client := fake.NewSimpleClientset(objects...)
			if test.apiError {
				client.PrependReactor("get", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("pod lookup unavailable")
				})
			}
			recovered, err := staticPodFailureRecovered(context.Background(), client, failure, test.events)
			if recovered != test.want || (err != nil) != test.apiError {
				t.Fatalf("recovered=%v err=%v, want recovered=%v error=%v", recovered, err, test.want, test.apiError)
			}
		})
	}
}

func TestStaticPodEventTime(t *testing.T) {
	t.Parallel()
	first := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	last := first.Add(time.Hour)
	for _, test := range []struct {
		name  string
		event eventsv1.Event
		want  time.Time
	}{
		{name: "event time", event: eventsv1.Event{EventTime: metav1.NewMicroTime(first)}, want: first},
		{name: "repeated failure uses latest series observation", event: eventsv1.Event{EventTime: metav1.NewMicroTime(first), Series: &eventsv1.EventSeries{LastObservedTime: metav1.NewMicroTime(last)}}, want: last},
		{name: "legacy timestamp", event: eventsv1.Event{DeprecatedLastTimestamp: metav1.NewTime(last)}, want: last},
		{name: "latest timestamp wins", event: eventsv1.Event{EventTime: metav1.NewMicroTime(last), Series: &eventsv1.EventSeries{LastObservedTime: metav1.NewMicroTime(first)}, DeprecatedLastTimestamp: metav1.NewTime(first)}, want: last},
		{name: "unknown time"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := staticPodEventTime(test.event); !got.Equal(test.want) {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}
