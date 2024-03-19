package watchclusteroperators

import (
	"sync"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type errorRecordingListWatcher struct {
	lw cache.ListerWatcher

	recorder monitorapi.RecorderWriter

	lock          sync.Mutex
	receivedError bool
}

func newErrorRecordingListWatcher(recorder monitorapi.RecorderWriter, lw cache.ListerWatcher) cache.ListerWatcher {
	return &errorRecordingListWatcher{
		lw:       lw,
		recorder: recorder,
	}
}

func (w *errorRecordingListWatcher) List(options metav1.ListOptions) (runtime.Object, error) {
	obj, err := w.lw.List(options)
	w.handle(err)
	return obj, err
}

func (w *errorRecordingListWatcher) Watch(options metav1.ListOptions) (watch.Interface, error) {
	obj, err := w.lw.Watch(options)
	w.handle(err)
	return obj, err
}

func (w *errorRecordingListWatcher) handle(err error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if err != nil {
		if !w.receivedError {

			i := monitorapi.NewInterval(monitorapi.APIServerClusterOperatorWatcher, monitorapi.Error).
				Locator(monitorapi.NewLocator().
					LocateServer("kube-apiserver", "", "", ""),
				).
				Message(monitorapi.NewMessage().
					Reason(monitorapi.FailedContactingAPIReason).
					HumanMessagef("failed contacting the API: %v", err),
				).
				Display().
				BuildNow()
			w.recorder.AddIntervals(i)
		}
		w.receivedError = true
	} else {
		w.receivedError = false
	}
}
