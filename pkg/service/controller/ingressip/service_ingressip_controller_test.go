package ingressip

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	informers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/registry/core/service/ipallocator"
)

const namespace = "ns"

func newController(t *testing.T, client *fake.Clientset, stopCh <-chan struct{}) *IngressIPController {
	_, ipNet, err := net.ParseCIDR("172.16.0.12/28")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		client = fake.NewSimpleClientset()
	}
	informerFactory := informers.NewSharedInformerFactory(client, controller.NoResyncPeriodFunc())
	controller := NewIngressIPController(
		informerFactory.Core().V1().Services().Informer(),
		client, ipNet, 10*time.Minute,
	)
	informerFactory.Start(stopCh)
	if !cache.WaitForCacheSync(stopCh, controller.hasSynced) {
		t.Fatalf("did not sync")
	}
	return controller
}

func controllerSetup(t *testing.T, startingObjects []runtime.Object, stopCh <-chan struct{}) (*fake.Clientset, *watch.FakeWatcher, *IngressIPController) {
	client := fake.NewSimpleClientset(startingObjects...)

	fakeWatch := watch.NewFake()
	client.PrependWatchReactor("*", clientgotesting.DefaultWatchReactor(fakeWatch, nil))

	client.PrependReactor("create", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		obj := action.(clientgotesting.CreateAction).GetObject()
		fakeWatch.Add(obj)
		return true, obj, nil
	})

	// Ensure that updates the controller makes are passed through to the watcher.
	client.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		obj := action.(clientgotesting.CreateAction).GetObject()
		fakeWatch.Modify(obj)
		return true, obj, nil
	})

	controller := newController(t, client, stopCh)

	return client, fakeWatch, controller
}

func newService(name, ip string, typeLoadBalancer bool) *v1.Service {
	serviceType := v1.ServiceTypeClusterIP
	if typeLoadBalancer {
		serviceType = v1.ServiceTypeLoadBalancer
	}
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1.ServiceSpec{
			Type: serviceType,
		},
	}
	if len(ip) > 0 {
		service.Status = v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: ip,
					},
				},
			},
		}
	}
	return service
}

func TestProcessInitialSync(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)
	c := newController(t, nil, stopCh)

	allocatedKey := "lb-allocated"
	allocatedIP := "172.16.0.1"
	services := []*v1.Service{
		newService("regular", "", false),
		newService(allocatedKey, allocatedIP, true),
		newService("lb-reallocate", "foo", true),
		newService("lb-unallocated", "", true),
	}
	for _, service := range services {
		c.enqueueChange(service, nil)
		c.cache.Add(service)
	}
	// Queue a change without caching it to validate that it is ignored
	c.enqueueChange(newService("ignored", "", true), nil)

	// Enqueue post-sync changes to validate that they are added back
	// to the queue without being processed.
	postSyncUpdate := services[0]
	c.enqueueChange(postSyncUpdate, postSyncUpdate)
	c.cache.Update(postSyncUpdate)
	postSyncAddition := newService("lb-post-sync-addition", "", true)
	c.enqueueChange(postSyncAddition, nil)
	c.cache.Add(postSyncAddition)

	c.processInitialSync()

	// Validate allocation
	expectedMap := map[string]string{
		allocatedIP: fmt.Sprintf("%s/%s", namespace, allocatedKey),
	}
	if !reflect.DeepEqual(c.allocationMap, expectedMap) {
		t.Errorf("Expected allocation map %v, got %v", expectedMap, c.allocationMap)
	}
	if !c.ipAllocator.Has(net.ParseIP(allocatedIP)) {
		t.Errorf("IP %v was not marked as allocated", allocatedIP)
	}

	// Validate queue contents
	expectedQueueLength := 5 // 3 from initial sync, 2 from post-sync changes
	if c.queue.Len() != expectedQueueLength {
		t.Errorf("Expected queue length of %d, got %d", expectedQueueLength, c.queue.Len())
	}
}

func TestWorkRequeuesWhenFull(t *testing.T) {
	tests := []struct {
		testName        string
		requeuedChange  bool
		requeuedService bool
		requeued        bool
	}{
		{
			testName: "Previously requeued change should be requeued",
			requeued: true,
		},
		{
			testName:        "The only pending allocation should be requeued",
			requeuedChange:  true,
			requeuedService: true,
			requeued:        true,
		},
		{
			testName:        "Already requeued allocation should not be requeued",
			requeuedService: true,
			requeued:        false,
		},
	}
	for _, test := range tests {
		stopCh := make(chan struct{})
		c := newController(t, nil, stopCh)
		c.changeHandler = func(change *serviceChange) error {
			return ipallocator.ErrFull
		}
		// Use a queue with no delay to avoid timing issues
		c.queue = workqueue.NewRateLimitingQueue(workqueue.NewMaxOfRateLimiter())
		change := &serviceChange{
			key:                "foo",
			requeuedAllocation: test.requeuedChange,
		}
		if test.requeuedService {
			c.requeuedAllocations.Insert(change.key)
		}
		c.queue.Add(change)

		c.work()

		requeued := (c.queue.Len() == 1)
		if test.requeued != requeued {
			t.Errorf("Expected requeued == %v, got %v", test.requeued, requeued)
		}
		close(stopCh)
	}
}

func TestProcessChange(t *testing.T) {
	tests := []struct {
		testName    string
		ip          string
		lb          bool
		deleted     bool
		allocatedIP string
		ipAllocated bool
	}{
		{
			testName: "Deleted service",
			deleted:  true,
		},
		{
			testName:    "Existing allocation",
			ip:          "172.16.0.1",
			lb:          true,
			allocatedIP: "172.16.0.1",
		},
		{
			testName:    "Needs allocation",
			lb:          true,
			ipAllocated: true,
		},
		{
			testName: "Needs deallocation",
			ip:       "172.16.0.1",
			lb:       false,
		},
	}
	for _, test := range tests {
		stopCh := make(chan struct{})
		c := newController(t, nil, stopCh)
		c.persistenceHandler = func(client kcoreclient.ServicesGetter, service *v1.Service, targetStatus bool) error {
			return nil
		}
		s := newService("svc", test.ip, test.lb)
		if !test.deleted {
			c.cache.Add(s)
		}
		key := fmt.Sprintf("%s/%s", namespace, s.Name)
		addAllocation := len(test.ip) > 0 && len(test.allocatedIP) == 0
		if addAllocation {
			c.allocationMap[test.ip] = key
		}
		change := &serviceChange{key: key}

		freeBefore := c.ipAllocator.Free()

		c.processChange(change)

		switch {
		case len(test.allocatedIP) > 0:
			if _, ok := c.allocationMap[test.allocatedIP]; !ok {
				t.Errorf("%s: %v was not allocated as expected", test.testName, test.allocatedIP)
			}
		case test.ipAllocated:
			if freeBefore == c.ipAllocator.Free() {
				t.Errorf("%s: ip was not allocated", test.testName)
			}
		case len(test.ip) > 0:
			if _, ok := c.allocationMap[test.ip]; ok {
				t.Errorf("%s: %v was not deallocated as expected", test.testName, test.ip)
			}
		}
		close(stopCh)
	}
}

func TestClearOldAllocation(t *testing.T) {
	tests := []struct {
		testName string
		oldIP    string
		newIP    string
		cleared  bool
	}{
		{
			testName: "No old allocation",
			oldIP:    "",
			newIP:    "foo",
		},
		{
			testName: "Unchanged allocation",
			oldIP:    "172.16.0.1",
			newIP:    "172.16.0.1",
		},
		{
			testName: "Old allocation should be cleared",
			oldIP:    "172.16.0.1",
			newIP:    "172.16.0.2",
			cleared:  true,
		},
	}
	for _, test := range tests {
		stopCh := make(chan struct{})
		c := newController(t, nil, stopCh)
		new := newService("new", test.newIP, true)
		old := newService("old", test.oldIP, true)
		if cleared := c.clearOldAllocation(new, old); test.cleared != cleared {
			t.Errorf("%s: expected cleared %v, got %v", test.testName, test.cleared, cleared)
		}
		close(stopCh)
	}
}

func TestRecordAllocationReallocates(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)
	c := newController(t, nil, stopCh)
	var persisted *v1.Service
	// Keep track of the last-persisted service
	c.persistenceHandler = func(client kcoreclient.ServicesGetter, service *v1.Service, targetStatus bool) error {
		persisted = service
		return nil
	}
	s := newService("bad-ip", "foo", true)
	key := fmt.Sprintf("%s/%s", namespace, s.Name)
	err := c.recordAllocation(s, key)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if persisted == nil {
		t.Errorf("Service was not persisted")
	}
	if len(c.allocationMap) != 1 {
		t.Errorf("Service ip was not reallocated")
	}
	if ingress := persisted.Status.LoadBalancer.Ingress; len(ingress) == 0 {
		t.Errorf("Ingress ip was not persisted")
	}
}

func TestAllocateReleasesOnPersistenceFailure(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)
	c := newController(t, nil, stopCh)
	expectedFree := c.ipAllocator.Free()
	expectedErr := errors.New("Persistence failure")
	c.persistenceHandler = func(client kcoreclient.ServicesGetter, service *v1.Service, targetStatus bool) error {
		return expectedErr
	}
	s := newService("svc", "", true)
	key := fmt.Sprintf("%s/%s", namespace, s.Name)
	err := c.allocate(s, key)
	if !reflect.DeepEqual(expectedErr, err) {
		t.Fatalf("Expected err %v, got %v", expectedErr, err)
	}
	if expectedFree != c.ipAllocator.Free() {
		t.Fatalf("IP wasn't released on error")
	}
}

func TestClearLocalAllocation(t *testing.T) {
	tests := []struct {
		testName     string
		key          string
		ip           string
		allocatedKey string
		cleared      bool
	}{
		{
			testName: "Invalid ip",
			ip:       "foo",
		},
		{
			testName: "IP not allocated",
			ip:       "172.16.0.1",
		},
		{
			testName:     "IP not allocated to service",
			key:          "foo",
			ip:           "172.16.0.1",
			allocatedKey: "bar",
		},
		{
			testName:     "Local ip allocation cleared",
			key:          "foo",
			ip:           "172.16.0.1",
			allocatedKey: "foo",
			cleared:      true,
		},
	}
	for _, test := range tests {
		stopCh := make(chan struct{})
		c := newController(t, nil, stopCh)
		if len(test.allocatedKey) > 0 {
			c.allocationMap[test.ip] = test.allocatedKey
			c.ipAllocator.Allocate(net.ParseIP(test.ip))
		}
		if cleared := c.clearLocalAllocation(test.key, test.ip); test.cleared != cleared {
			t.Errorf("%s: expected cleared %v, got %v", test.testName, test.cleared, cleared)
		} else if cleared {
			if _, ok := c.allocationMap[test.ip]; ok {
				t.Errorf("%s: allocation map was not cleared", test.testName)
			}
			if c.ipAllocator.Has(net.ParseIP(test.ip)) {
				t.Errorf("%s: ip %v is still allocated", test.testName, test.ip)
			}
		}
		close(stopCh)
	}
}

func TestEnsureExternalIPRespectsNonIngress(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)
	c := newController(t, nil, stopCh)
	c.persistenceHandler = func(client kcoreclient.ServicesGetter, service *v1.Service, targetStatus bool) error {
		return nil
	}
	ingressIP := "172.16.0.1"
	s := newService("foo", ingressIP, true)
	externalIP := "172.16.1.1"
	s.Spec.ExternalIPs = append(s.Spec.ExternalIPs, externalIP)
	c.ensureExternalIP(s, s.Name, ingressIP)
	expectedExternalIPs := []string{externalIP, ingressIP}
	externalIPs := s.Spec.ExternalIPs
	if !reflect.DeepEqual(expectedExternalIPs, externalIPs) {
		t.Errorf("Expected ExternalIPs %v, got %v", expectedExternalIPs, externalIPs)
	}
}

func TestAllocateIP(t *testing.T) {
	tests := []struct {
		testName    string
		requestedIP string
		allocated   bool
		asRequested bool
	}{
		{
			testName:    "No requested ip",
			requestedIP: "",
			asRequested: false,
		},
		{
			testName:    "Invalid ip",
			requestedIP: "foo",
			asRequested: false,
		},
		{
			testName:    "IP not available",
			requestedIP: "172.16.0.1",
			allocated:   true,
			asRequested: false,
		},
		{
			testName:    "Available",
			requestedIP: "172.16.0.1",
			asRequested: true,
		},
	}
	for _, test := range tests {
		stopCh := make(chan struct{})
		controller := newController(t, nil, stopCh)
		if test.allocated {
			ip := net.ParseIP(test.requestedIP)
			controller.ipAllocator.Allocate(ip)
		}
		// Expect no error for these
		ip, err := controller.allocateIP(test.requestedIP)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if test.asRequested && ip.String() != test.requestedIP {
			t.Errorf("%s: expected %s but got %s", test.testName, test.requestedIP, ip.String())
		}
		if !test.asRequested && ip.String() == test.requestedIP {
			t.Errorf("%s: did not expect %s", test.testName, test.requestedIP)
		}
		close(stopCh)
	}
}

func TestRecordLocalAllocation(t *testing.T) {
	key := "svc1"
	ip := "172.16.0.1"
	otherKey := "svc2"
	tests := []struct {
		testName      string
		allocationMap map[string]string
		ip            string
		reallocate    bool
		errExpected   bool
	}{
		{
			testName:    "Invalid ip",
			ip:          "foo",
			reallocate:  true,
			errExpected: true,
		},
		{
			testName: "Allocation exists for service",
			allocationMap: map[string]string{
				ip: key,
			},
			ip: ip,
		},
		{
			testName: "Allocation exists for another service",
			allocationMap: map[string]string{
				ip: otherKey,
			},
			ip:          ip,
			reallocate:  true,
			errExpected: true,
		},
		{
			testName:    "IP not in range",
			ip:          "172.16.1.1",
			reallocate:  true,
			errExpected: true,
		},
		{
			testName: "Allocation successful",
			ip:       "172.16.0.1",
		},
	}
	for _, test := range tests {
		stopCh := make(chan struct{})
		c := newController(t, nil, stopCh)
		if test.allocationMap != nil {
			c.allocationMap = test.allocationMap
			for ipString := range test.allocationMap {
				c.ipAllocator.Allocate(net.ParseIP(ipString))
			}
		}

		reallocate, err := c.recordLocalAllocation(key, test.ip)

		if test.reallocate != reallocate {
			t.Errorf("%s: expected reallocate == %v but got %v", test.testName, test.reallocate, reallocate)
		}
		switch {
		case test.errExpected && (err == nil):
			t.Errorf("%s: expected error but didn't see one", test.testName)
		case !test.errExpected && (err != nil):
			t.Errorf("%s: saw unexpected error: %v", test.testName, err)
		}

		// Ensure allocation was successfully recorded
		checkAllocation := !test.reallocate && !test.errExpected
		if checkAllocation {
			ipKey, ok := c.allocationMap[test.ip]
			inMap := ok && ipKey == key
			inAllocator := c.ipAllocator.Has(net.ParseIP(test.ip))
			if !(inMap && inAllocator) {
				t.Errorf("%s: allocation not recorded", test.testName)
			}
		}
		close(stopCh)
	}
}

func TestClearPersistedAllocation(t *testing.T) {
	tests := []struct {
		testName         string
		persistenceError error
		ingressIPCount   int
	}{
		{
			testName:         "Status not cleared if external ip not removed",
			persistenceError: errors.New(""),
			ingressIPCount:   1,
		},
		{
			testName: "Status cleared",
		},
	}
	for _, test := range tests {
		stopCh := make(chan struct{})
		c := newController(t, nil, stopCh)
		var persistedService *v1.Service
		c.persistenceHandler = func(client kcoreclient.ServicesGetter, service *v1.Service, targetStatus bool) error {
			// Save the last persisted service
			persistedService = service
			return test.persistenceError
		}
		ip := "172.16.0.1"
		s := newService("svc", ip, true)
		// Add other external ips to ensure they are not affected by controller
		s.Spec.ExternalIPs = []string{"172.16.1.1", ip, "172.16.1.2"}
		key := fmt.Sprintf("%s/%s", namespace, s.Name)
		c.clearPersistedAllocation(s, key, "")

		expectedExternalIPs := []string{"172.16.1.1", "172.16.1.2"}
		externalIPs := persistedService.Spec.ExternalIPs
		if !reflect.DeepEqual(expectedExternalIPs, externalIPs) {
			t.Errorf("%s: Expected ExternalIPs %v, got %v", test.testName, expectedExternalIPs, externalIPs)
		}
		ingressIPCount := len(persistedService.Status.LoadBalancer.Ingress)
		if test.ingressIPCount != ingressIPCount {
			t.Errorf("%s: Expected %d ingress ips, got %d", test.testName, test.ingressIPCount, ingressIPCount)
		}
		close(stopCh)
	}
}

// TestBasicControllerFlow validates controller start, initial sync
// processing, and post-sync processing.
func TestBasicControllerFlow(t *testing.T) {
	startingObjects := []runtime.Object{
		newService("lb-unallocated", "", true),
	}

	stopChannel := make(chan struct{})
	defer close(stopChannel)

	_, fakeWatch, controller := controllerSetup(t, startingObjects, stopChannel)

	updated := make(chan bool)
	deleted := make(chan bool)

	controller.changeHandler = func(change *serviceChange) error {
		defer func() {
			if len(change.key) == 0 {
				deleted <- true
			} else if change.oldService != nil {
				updated <- true
			}
		}()

		err := controller.processChange(change)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		return err
	}

	go controller.Run(stopChannel)

	waitForUpdate := func(msg string) {
		t.Logf("waiting for: %v", msg)
		select {
		case <-updated:
		case <-time.After(time.Duration(30 * time.Second)):
			t.Fatalf("failed to see: %v", msg)
		}
	}

	waitForUpdate("spec update")
	waitForUpdate("status update")

	fakeWatch.Delete(startingObjects[0])

	t.Log("waiting for the service to be deleted")
	select {
	case <-deleted:
	case <-time.After(time.Duration(30 * time.Second)):
		t.Fatalf("failed to see expected service deletion")
	}
}
