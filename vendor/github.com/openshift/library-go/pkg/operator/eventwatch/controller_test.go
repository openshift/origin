package eventwatch

import (
	"context"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/library-go/pkg/operator/events"
)

func TestControllerEventCount(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	eventRecorder := events.NewRecorder(kubeClient.CoreV1().Events("test"), "test-operator", &corev1.ObjectReference{
		Namespace: "test",
	})
	informer := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("test"))

	processCount := 0
	processChan := make(chan struct{})
	var processCountLock sync.Mutex
	b := New().WithEventHandler("test", "TestReason", func(event *corev1.Event) error {
		defer func() {
			processCountLock.Lock()
			processCount++
			processCountLock.Unlock()
			processChan <- struct{}{}
		}()
		t.Logf("process(%d): %#v", event.Count, event.Annotations)
		return nil
	})
	controller := b.ToController(informer, kubeClient.CoreV1(), eventRecorder)

	ctx, shutdown := context.WithCancel(context.Background())
	defer shutdown()

	informer.Start(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.Core().V1().Events().Informer().HasSynced)
	go controller.Run(ctx, 1)

	time.Sleep(1 * time.Second) // give controller some time to start workers...

	event, _ := kubeClient.CoreV1().Events("test").Create(ctx, &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-event",
			Namespace: "test",
		},
		Reason: "TestReason",
		Count:  1,
	}, metav1.CreateOptions{})

	waitForAcknowledged := func(count string) error {
		return wait.PollImmediate(100*time.Millisecond, 10*time.Second, func() (done bool, err error) {
			event, err := kubeClient.CoreV1().Events("test").Get(ctx, "test-event", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			c, ok := event.Annotations[eventAckAnnotationName]
			if !ok {
				return false, nil
			}
			t.Logf("acknowledged: %q", c)
			return c == count, nil
		})
	}
	<-processChan
	if err := waitForAcknowledged("1"); err != nil {
		t.Fatal(err)
	}

	eventFirstUpdate := event.DeepCopy()
	eventFirstUpdate.Count = 2
	updatedEvent, _ := kubeClient.CoreV1().Events("test").Update(ctx, eventFirstUpdate, metav1.UpdateOptions{})
	<-processChan
	if err := waitForAcknowledged("2"); err != nil {
		t.Fatal(err)
	}

	eventSecondUpdate := updatedEvent.DeepCopy()
	eventSecondUpdate.Count = 3
	kubeClient.CoreV1().Events("test").Update(ctx, eventSecondUpdate, metav1.UpdateOptions{})
	<-processChan
	if err := waitForAcknowledged("3"); err != nil {
		t.Fatal(err)
	}

	processCountLock.Lock()
	currentCount := processCount
	processCountLock.Unlock()
	if currentCount != 3 {
		t.Errorf("expected process() called three times, got %d", currentCount)
	}
}

func TestController(t *testing.T) {
	tests := []struct {
		name                 string
		handlers             []eventHandler
		sendEvents           func(recorder events.Recorder)
		expectedEventsKeys   []string
		expectedProcessCount int
		evalActions          func(t *testing.T, actions []ktesting.Action)
	}{
		{
			name: "got test reason",
			handlers: []eventHandler{
				{
					reason:    "TestReason",
					namespace: "test",
				},
			},
			sendEvents: func(recorder events.Recorder) {
				recorder.Warningf("TestReason", "Test")
			},
			expectedProcessCount: 1,
			expectedEventsKeys: []string{
				"test/name/TestReason",
			},
		},
		{
			name: "ignore other events",
			handlers: []eventHandler{
				{
					reason:    "TestReason",
					namespace: "test",
				},
			},
			sendEvents: func(recorder events.Recorder) {
				recorder.Warningf("TestReason", "Test")
				recorder.Warningf("OtherEvent", "Test")
			},
			expectedProcessCount: 1,
			expectedEventsKeys: []string{
				"test/name/TestReason",
			},
		},
		{
			name: "test reason event acknowledged",
			handlers: []eventHandler{
				{
					reason:    "TestReason",
					namespace: "test",
				},
			},
			sendEvents: func(recorder events.Recorder) {
				recorder.Warningf("TestReason", "Test")
				recorder.Warningf("TestReason", "Test")
				recorder.Warningf("TestReason", "Test")
			},
			expectedProcessCount: 1,
			expectedEventsKeys: []string{
				"test/name/TestReason",
			},
			evalActions: func(t *testing.T, actions []ktesting.Action) {
				acked := false
				count := ""
				for _, action := range actions {
					if action.GetVerb() == "update" {
						update := action.(ktesting.UpdateAction)
						event, ok := update.GetObject().(*corev1.Event)
						if ok && event.Reason == "TestReason" && event.Annotations != nil {
							count, acked = event.Annotations[eventAckAnnotationName]
						}
					}
				}
				if !acked {
					t.Errorf("expected event to be acknowledged")
				}
				if count != "1" {
					t.Errorf("expected count to be 1, got %q", count)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			eventProcessedChan := make(chan string)
			b := New()

			processCount := 0
			var processCountLock sync.Mutex
			for _, h := range test.handlers {
				b = b.WithEventHandler(h.namespace, h.reason, func(event *corev1.Event) error {
					defer func() {
						processCountLock.Lock()
						processCount++
						processCountLock.Unlock()
					}()
					var err error
					if h.process != nil {
						err = h.process(event)
					}
					key := eventKeyFunc(event.Namespace, "name", event.Reason) // name is random
					if key == "" {
						close(eventProcessedChan)
					}
					eventProcessedChan <- key
					return err
				})
			}

			kubeClient := fake.NewSimpleClientset()
			eventRecorder := events.NewRecorder(kubeClient.CoreV1().Events("test"), "test-operator", &corev1.ObjectReference{
				Namespace: "test",
			})
			informer := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("test"))

			controller := b.ToController(informer, kubeClient.CoreV1(), eventRecorder)

			ctx, shutdown := context.WithCancel(context.Background())
			defer shutdown()

			informer.Start(ctx.Done())
			go controller.Run(ctx, 1)

			test.sendEvents(eventRecorder)

			recvKeys := sets.NewString()
			finish := false
			for !finish {
				select {
				case eventKey := <-eventProcessedChan:
					recvKeys.Insert(eventKey)
					if len(test.expectedEventsKeys) == recvKeys.Len() {
						finish = true
						break
					}
				case <-time.After(30 * time.Second):
					t.Fatal("timeout")
				}
			}

			if !recvKeys.Equal(sets.NewString(test.expectedEventsKeys...)) {
				t.Errorf("received keys (%#v) does not have all expected keys: %#v", recvKeys.List(), test.expectedEventsKeys)
			}

			if test.evalActions != nil {
				test.evalActions(t, kubeClient.Actions())
			}
			if test.expectedProcessCount > 0 {
				processCountLock.Lock()
				if test.expectedProcessCount != processCount {
					t.Errorf("expected %d process() calls, got %d", test.expectedProcessCount, processCount)
				}
				processCountLock.Unlock()
			}

		})
	}
}
