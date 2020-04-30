package events

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

var fakeObjectReference = &v1.ObjectReference{
	Kind:       "Deployment",
	Namespace:  "operator-namespace",
	Name:       "operator",
	UID:        "33ff5131-0396-4536-8ef0-8f31b397adee",
	APIVersion: "apps/v1",
}

func TestUpstreamRecorder_Correlator(t *testing.T) {
	tests := []struct {
		name       string
		options    record.CorrelatorOptions
		runEvents  func(Recorder)
		evalEvents func(*v1.EventList, *testing.T)
	}{
		{
			name:    "correlate events similar messages with default event recorder",
			options: record.CorrelatorOptions{},
			runEvents: func(r Recorder) {
				for i := 0; i <= 30; i++ {
					time.Sleep(10 * time.Millisecond)
					r.Event("TestReason", fmt.Sprintf("test message %d", i))
				}
			},
			evalEvents: func(events *v1.EventList, t *testing.T) {
				if len(events.Items) < 10 {
					t.Errorf("expected 10 events, got %d", len(events.Items))
				}
				if lastEventMessage := events.Items[len(events.Items)-1].Message; !strings.Contains(lastEventMessage, "combined from similar events") {
					t.Errorf("expected last event to be combined, got %q", lastEventMessage)
				}
			},
		},
		{
			name:    "do not correlate events with similar messages with operator options",
			options: RecommendedClusterSingletonCorrelatorOptions(),
			runEvents: func(r Recorder) {
				for i := 0; i <= 30; i++ {
					time.Sleep(10 * time.Millisecond)
					r.Event("TestReason", fmt.Sprintf("test message %d", i))
				}
			},
			evalEvents: func(events *v1.EventList, t *testing.T) {
				if len(events.Items) < 30 {
					t.Errorf("expected 30 events, got %d", len(events.Items))
				}
				for _, e := range events.Items {
					if strings.Contains(e.Message, "combined") {
						t.Errorf("expected no combined messaged, got %q", e.Message)
						break
					}
				}
			},
		},
		{
			name:    "correlate events with same messages with operator options",
			options: RecommendedClusterSingletonCorrelatorOptions(),
			runEvents: func(r Recorder) {
				for i := 0; i <= 30; i++ {
					time.Sleep(10 * time.Millisecond)
					r.Event("TestReason", "test message")
				}
			},
			evalEvents: func(events *v1.EventList, t *testing.T) {
				if len(events.Items) != 1 {
					t.Errorf("expected 1 event, got %d", len(events.Items))
				}
				if count := events.Items[0].Count; count < 30 {
					t.Errorf("expected the event have count of 30, got %d", count)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			testRecorder := NewKubeRecorderWithOptions(client.CoreV1().Events("operator-namespace"), test.options, "test", fakeObjectReference).WithComponentSuffix("suffix")
			test.runEvents(testRecorder)

			events, err := client.CoreV1().Events("operator-namespace").List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				t.Fatal(err)
			}
			test.evalEvents(events, t)
		})
	}

}

/*
// TODO: This test is racy, because upstream event recorder is non-blocking... which means the all events are created as go-routines with non-fallback recorder...
func TestUpstreamRecorder_Shutdown(t *testing.T) {
	for testCount := 1; testCount != 10; testCount++ {
		t.Run(fmt.Sprintf("test_%d", testCount), func(t *testing.T) {
			client := fake.NewSimpleClientset()
			fallbackClient := fake.NewSimpleClientset()
			recorder := NewKubeRecorder(client.CoreV1().Events("operator-namespace"), RecommendedClusterSingletonCorrelatorOptions(), "test", fakeObjectReference)
			recorder.(*upstreamRecorder).fallbackRecorder = NewKubeRecorder(fallbackClient.CoreV1().Events("operator-namespace"), RecommendedClusterSingletonCorrelatorOptions(), "test", fakeObjectReference)

			eventsSendChan := make(chan struct{})
			eventsDoneChan := make(chan struct{})
			go func() {
				defer close(eventsDoneChan)
				counter := 0
				for i := 0; i <= 50; i++ {
					time.Sleep(5 * time.Millisecond)
					recorder.Eventf(fmt.Sprintf("TestReason%d", i), "test message %d", i)
					counter++
					if counter == 10 {
						// at this point the recorder should switch to fallback client
						close(eventsSendChan)
					}
				}
			}()

			<-eventsSendChan
			recorder.Shutdown()
			<-eventsDoneChan

			t.Logf("client actions: %d", len(client.Actions()))
			t.Logf("fallback client actions: %d", len(fallbackClient.Actions()))
		})
	}
}
*/
