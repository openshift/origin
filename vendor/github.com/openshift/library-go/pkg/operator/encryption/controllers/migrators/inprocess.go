package migrators

import (
	"context"
	"fmt"
	"sync"

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
}

func (m *InProcessMigrator) EnsureMigration(gr schema.GroupResource, writeKey string) (finished bool, result error, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// finished?
	migration := m.running[gr]
	if migration != nil && migration.writeKey == writeKey {
		if migration.result == nil {
			return false, nil, nil
		}
		return true, *migration.result, nil
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
		return false, nil, err
	}

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	m.running[gr] = &inProcessMigration{
		stopCh:   stopCh,
		doneCh:   doneCh,
		writeKey: writeKey,
	}

	go m.runMigration(gr.WithVersion(v), writeKey, stopCh, doneCh)

	return false, nil, nil
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
		allResource, err := d.List(opts)
		if err != nil {
			return nil, err // TODO this can wedge on resource expired errors with large overall list
		}
		for _, obj := range allResource.Items { // TODO parallelize for-loop
			_, updateErr := d.Namespace(obj.GetNamespace()).Update(&obj, metav1.UpdateOptions{})
			errs = append(errs, updateErr)
		}
		allResource.Items = nil // do not accumulate items, this fakes the visitor pattern
		return allResource, nil // leave the rest of the list intact to preserve continue token
	}))

	listPager.FullListIfExpired = false // prevent memory explosion from full list
	_, listErr := listPager.List(ctx, metav1.ListOptions{})
	errs = append(errs, listErr)
	result = utilerrors.FilterOut(utilerrors.NewAggregate(errs), errors.IsNotFound, errors.IsConflict)
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
