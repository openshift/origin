package migrators

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/pager"
	"k8s.io/klog"
)

func NewInProcessMigrator(dynamicClient dynamic.Interface, discoveryClient discovery.ServerResourcesInterface) *InProcessMigrator {
	return &InProcessMigrator{
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
		running:         map[schema.GroupResource]*inProcessMigration{},
	}
}

// InProcessMigrator runs migration in-process using paging.
type InProcessMigrator struct {
	dynamicClient   dynamic.Interface
	discoveryClient discovery.ServerResourcesInterface

	lock    sync.Mutex
	running map[schema.GroupResource]*inProcessMigration

	handler cache.ResourceEventHandler
}

type inProcessMigration struct {
	stopCh   chan<- struct{}
	doneCh   <-chan struct{}
	writeKey string

	// non-nil when finished. *result==nil means "no error"
	result *error
	// when did it finish
	timestamp time.Time
}

func (m *InProcessMigrator) EnsureMigration(gr schema.GroupResource, writeKey string) (finished bool, result error, ts time.Time, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// finished?
	migration := m.running[gr]
	if migration != nil && migration.writeKey == writeKey {
		if migration.result == nil {
			return false, nil, time.Time{}, nil
		}
		return true, *migration.result, migration.timestamp, nil
	}

	// different key?
	if migration != nil && migration.result == nil {
		klog.V(4).Infof("interrupting running migration for resource %v and write key %q", gr, migration.writeKey)
		close(migration.stopCh)

		// give go routine time to update the result
		m.lock.Unlock()
		<-migration.doneCh
		m.lock.Lock()
	}

	v, err := preferredResourceVersion(m.discoveryClient, gr)
	if err != nil {
		return false, nil, time.Time{}, err
	}

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	m.running[gr] = &inProcessMigration{
		stopCh:   stopCh,
		doneCh:   doneCh,
		writeKey: writeKey,
	}

	go m.runMigration(gr.WithVersion(v), writeKey, stopCh, doneCh)

	return false, nil, time.Time{}, nil
}

func (m *InProcessMigrator) runMigration(gvr schema.GroupVersionResource, writeKey string, stopCh <-chan struct{}, doneCh chan<- struct{}) {
	var result error

	defer close(doneCh)
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				result = err
			} else {
				result = fmt.Errorf("panic: %v", r)
			}
		}

		m.lock.Lock()
		defer m.lock.Unlock()
		migration := m.running[gvr.GroupResource()]
		if migration == nil || migration.writeKey != writeKey {
			// ok, this is not us. Should never happen.
			return
		}

		migration.result = &result
		migration.timestamp = time.Now()

		m.handler.OnAdd(&corev1.Secret{}) // fake secret to trigger event loop of controller
	}()

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	go func() {
		<-stopCh
		cancelFn()
	}()

	d := m.dynamicClient.Resource(gvr)
	var errs []error
	listPager := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (runtime.Object, error) {
		for {
			allResource, err := d.List(opts)
			if err != nil {
				klog.Warningf("List of %v failed: %v", gvr, err)
				if errors.IsResourceExpired(err) {
					return nil, err // the pager will handle that
				} else if retryable := canRetry(err); retryable == nil || *retryable == false {
					return nil, err // not retryable or we don't know. Return error and controller will restart migration.
				} else {
					if seconds, delay := errors.SuggestsClientDelay(err); delay {
						time.Sleep(time.Duration(seconds) * time.Second)
					}
					klog.V(4).Infof("Relisting %v after retryable error: %v", gvr, err)
					continue
				}
			}

			klog.V(4).Infof("Migrating %d objects of %v", len(allResource.Items), gvr)

		nextItem:
			for _, obj := range allResource.Items { // TODO parallelize for-loop
				for {
					_, updateErr := d.Namespace(obj.GetNamespace()).Update(&obj, metav1.UpdateOptions{})
					if updateErr == nil || errors.IsNotFound(updateErr) || errors.IsConflict(updateErr) {
						continue nextItem
					}
					if retryable := canRetry(updateErr); retryable == nil || *retryable == false {
						klog.Warningf("Update of %s/%s failed: %v", obj.GetNamespace(), obj.GetName(), updateErr)
						errs = append(errs, updateErr) // not retryable or we don't know. Return error and controller will restart migration.
						break
					}
					if seconds, delay := errors.SuggestsClientDelay(updateErr); delay {
						klog.V(4).Infof("Sleeping %ds while updating %s/%s of type %v after retryable error: %v", seconds, obj.GetNamespace(), obj.GetName(), gvr, updateErr)
						time.Sleep(time.Duration(seconds) * time.Second)
					}
				}
			}

			klog.V(4).Infof("Migration of %d objects of %v finished", len(allResource.Items), gvr)

			allResource.Items = nil // do not accumulate items, this fakes the visitor pattern
			return allResource, nil // leave the rest of the list intact to preserve continue token
		}
	}))

	listPager.FullListIfExpired = false // prevent memory explosion from full list
	_, listErr := listPager.List(ctx, metav1.ListOptions{})
	errs = append(errs, listErr)
	result = utilerrors.NewAggregate(errs)
}

func (m *InProcessMigrator) PruneMigration(gr schema.GroupResource) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	migration := m.running[gr]
	delete(m.running, gr)

	// finished?
	if migration != nil && migration.result == nil {
		close(migration.stopCh)

		// give go routine time to update the result
		m.lock.Unlock()
		<-migration.doneCh
		m.lock.Lock()
	}

	return nil
}

func (m *InProcessMigrator) AddEventHandler(handler cache.ResourceEventHandler) []cache.InformerSynced {
	m.handler = handler
	return nil
}

func preferredResourceVersion(c discovery.ServerResourcesInterface, gr schema.GroupResource) (string, error) {
	resourceLists, discoveryErr := c.ServerPreferredResources() // safe to ignore error
	for _, resourceList := range resourceLists {
		groupVersion, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			return "", err
		}
		if groupVersion.Group != gr.Group {
			continue
		}
		for _, resource := range resourceList.APIResources {
			if (len(resource.Group) == 0 || resource.Group == gr.Group) && resource.Name == gr.Resource {
				if len(resource.Version) > 0 {
					return resource.Version, nil
				}
				return groupVersion.Version, nil
			}
		}
	}
	return "", fmt.Errorf("failed to find version for %s, discoveryErr=%v", gr, discoveryErr)
}
