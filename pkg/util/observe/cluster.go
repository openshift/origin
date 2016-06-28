package observe

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/net/context"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/storage"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/golang/glog"
)

type clusterResourceVersionObserver struct {
	versioner        storage.Versioner
	watchers         []rest.Watcher
	successThreshold int
}

// NewClusterObserver returns a ResourceVersionObserver that watches for the specified resourceVersion on all of the provided watchers.
// If at least successThreshold watchers observe the resourceVersion within the timeout, no error is returned.
func NewClusterObserver(versioner storage.Versioner, watchers []rest.Watcher, successThreshold int) ResourceVersionObserver {
	return &clusterResourceVersionObserver{
		versioner:        versioner,
		watchers:         watchers,
		successThreshold: successThreshold,
	}
}

func (c *clusterResourceVersionObserver) ObserveResourceVersion(resourceVersion string, timeout time.Duration) error {
	if len(c.watchers) == 0 {
		return nil
	}

	wg := &sync.WaitGroup{}
	backendErrors := make([]error, len(c.watchers), len(c.watchers))
	for i, watcher := range c.watchers {
		wg.Add(1)
		go func(i int, watcher rest.Watcher) {
			defer utilruntime.HandleCrash()
			defer wg.Done()
			backendErrors[i] = watchForResourceVersion(c.versioner, watcher, resourceVersion, timeout)
		}(i, watcher)
	}

	glog.V(5).Infof("waiting for resourceVersion %s to be distributed", resourceVersion)
	wg.Wait()

	successes := 0
	for _, err := range backendErrors {
		if err == nil {
			successes++
		} else {
			glog.V(4).Infof("error verifying resourceVersion %s: %v", resourceVersion, err)
		}
	}
	glog.V(5).Infof("resourceVersion %s was distributed to %d etcd cluster members (out of %d)", resourceVersion, successes, len(c.watchers))

	if successes >= c.successThreshold {
		return nil
	}

	return fmt.Errorf("resourceVersion %s was observed on %d cluster members (threshold %d): %v", resourceVersion, successes, c.successThreshold, backendErrors)
}

// watchForResourceVersion watches for an Add/Modify event matching the given resourceVersion.
// If an error, timeout, or unexpected event is received, an error is returned.
// If an add/modify event is observed with the correct resource version, nil is returned.
func watchForResourceVersion(versioner storage.Versioner, watcher rest.Watcher, resourceVersion string, timeout time.Duration) error {
	// Watch from the previous resource version, so the first watch event is the desired version
	previousVersion, err := previousResourceVersion(versioner, resourceVersion)
	if err != nil {
		return err
	}

	w, err := watcher.Watch(context.TODO(), &kapi.ListOptions{ResourceVersion: previousVersion})
	if err != nil {
		return fmt.Errorf("error verifying resourceVersion %s: %v", resourceVersion, err)
	}
	defer w.Stop()

	select {
	case event := <-w.ResultChan():
		if event.Type != watch.Added && event.Type != watch.Modified {
			return fmt.Errorf("unexpected watch event verifying resourceVersion %s: %q", resourceVersion, event.Type)
		}
		if event.Object == nil {
			return fmt.Errorf("unexpected watch event verifying resourceVersion %s: object was nil", resourceVersion)
		}
		accessor, err := meta.Accessor(event.Object)
		if err != nil {
			return err
		}
		actualResourceVersion := accessor.GetResourceVersion()
		if actualResourceVersion != resourceVersion {
			return fmt.Errorf("unexpected watch event verifying resourceVersion %s: resource version was %s)", resourceVersion, actualResourceVersion)
		}
		return nil

	case <-time.After(timeout):
		return fmt.Errorf("timeout verifying resourceVersion %s", resourceVersion)
	}
}

// previousResourceVersion returns the resource version one prior to the given resourceVersion.
// The first event seen by a watch started at the returned version should be the create/update of the object.
func previousResourceVersion(v storage.Versioner, resourceVersion string) (string, error) {
	// Any API object will do. We'll just use an Event
	e := &kapi.Event{}
	e.ResourceVersion = resourceVersion
	version, err := v.ObjectResourceVersion(e)
	if err != nil {
		return "", err
	}
	v.UpdateObject(e, version-1)
	return e.ResourceVersion, nil
}
