package watch

import (
	"time"

	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"
)

type WatcherFunc func(string) (watch.Interface, error)

// RetryWatchUntil starts a watch and handle the watch events using the conditionFunc. In
// case the watch is closed, this function will re-open it and continue handling events
// from the last resourceVersion received.
// The timeout can be set to limit the amount of time the conditionFunc should report
// results.
func RetryWatchUntil(watcher WatcherFunc, conditionFunc watch.ConditionFunc, timeout time.Duration) (bool, error) {
	type watchResult struct {
		err    error
		result bool
	}
	eventChan := make(chan watch.Event)
	stopChan := make(chan struct{})
	resultChan := make(chan watchResult)

	// This go-routine open the watcher and stream the events from the result channel of the
	// watcher to an eventChan. When the watcher window is closed (because it reached the
	// default limit of 1000 events) the watcher is re-opened with the last known resource
	// version of the object. The watcher stops when the stopChan is closed.
	lastResourceVersion := "0"
	go func() {
		defer close(eventChan)
		for {
			watch, err := watcher(lastResourceVersion)
			if err != nil {
				resultChan <- watchResult{err: err}
				return
			}
			for {
				retryWatcher := false
				select {
				case <-stopChan:
					return
				case event, ok := <-watch.ResultChan():
					if !ok {
						// the watcher was closed, but we will reopen it
						retryWatcher = true
						break
					}
					meta, err := meta.Accessor(event.Object)
					if err != nil {
						resultChan <- watchResult{err: err}
						return
					}
					lastResourceVersion = meta.GetResourceVersion()
					eventChan <- event
				}
				if retryWatcher {
					break
				}
			}
		}
	}()

	// This go routine reads from the eventChan and apply the condition function on the
	// events received from this channel. When the condition function return error or done,
	// then the result is reported to resultChan and this go routine exists.
	// Users might also set the timeout to say how long they want to wait for the condition
	// function.
	go func() {
		for {
			select {
			case <-time.After(timeout):
				resultChan <- watchResult{err: wait.ErrWaitTimeout}
				return
			case event, _ := <-eventChan:
				done, err := conditionFunc(event)
				if err != nil {
					resultChan <- watchResult{err: err}
					return
				}
				if done {
					resultChan <- watchResult{result: true}
					return
				}
				continue
			}
		}
	}()

	r := <-resultChan
	close(stopChan)
	return r.result, r.err
}
