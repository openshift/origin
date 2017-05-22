/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api/v1"
	externalclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset/fake"
	einformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	soptions "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
	"k8s.io/kubernetes/plugin/pkg/scheduler"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api/latest"
	"k8s.io/kubernetes/plugin/pkg/scheduler/core"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"

	// register algorithm providers
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	ccapi "github.com/kubernetes-incubator/cluster-capacity/pkg/api"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/record"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/restclient/external"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/store"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/strategy"
)

type ClusterCapacity struct {
	// caches modified by emulation strategy
	resourceStore store.ResourceStore

	// emulation strategy
	strategy strategy.Strategy

	// fake kube client
	externalkubeclient *externalclientset.Clientset

	informerFactory einformers.SharedInformerFactory

	// fake rest clients
	coreRestClient *external.RESTClient

	// schedulers
	schedulers       map[string]*scheduler.Scheduler
	schedulerConfigs map[string]*scheduler.Config
	defaultScheduler string

	// pod to schedule
	simulatedPod     *v1.Pod
	lastSimulatedPod *v1.Pod
	maxSimulated     int
	simulated        int
	status           Status
	report           *ClusterCapacityReview

	// analysis limitation
	informerStopCh chan struct{}

	// stop the analysis
	stop      chan struct{}
	stopMux   sync.RWMutex
	stopped   bool
	closedMux sync.RWMutex
	closed    bool
}

// capture all scheduled pods with reason why the analysis could not continue
type Status struct {
	Pods       []*v1.Pod
	StopReason string
}

func (c *ClusterCapacity) Report() *ClusterCapacityReview {
	if c.report == nil {
		// Preparation before pod sequence scheduling is done
		pods := make([]*v1.Pod, 0)
		pods = append(pods, c.simulatedPod)
		c.report = GetReport(pods, c.status)
		c.report.Spec.Replicas = int32(c.maxSimulated)
	}

	return c.report
}

func (c *ClusterCapacity) SyncWithClient(client externalclientset.Interface) error {
	for _, resource := range c.resourceStore.Resources() {
		var listWatcher *cache.ListWatch
		if resource == ccapi.ReplicaSets {
			listWatcher = cache.NewListWatchFromClient(client.Extensions().RESTClient(), resource.String(), metav1.NamespaceAll, fields.ParseSelectorOrDie(""))
		} else {
			listWatcher = cache.NewListWatchFromClient(client.Core().RESTClient(), resource.String(), metav1.NamespaceAll, fields.ParseSelectorOrDie(""))
		}

		options := metav1.ListOptions{ResourceVersion: "0"}
		list, err := listWatcher.List(options)
		if err != nil {
			return fmt.Errorf("Failed to list objects: %v", err)
		}

		listMetaInterface, err := meta.ListAccessor(list)
		if err != nil {
			return fmt.Errorf("Unable to understand list result %#v: %v", list, err)
		}
		resourceVersion := listMetaInterface.GetResourceVersion()

		items, err := meta.ExtractList(list)
		if err != nil {
			return fmt.Errorf("Unable to understand list result %#v (%v)", list, err)
		}
		found := make([]interface{}, 0, len(items))
		for _, item := range items {
			found = append(found, item)
		}
		err = c.resourceStore.Replace(resource, found, resourceVersion)
		if err != nil {
			return fmt.Errorf("Unable to store %s list result: %v", resource, err)
		}
	}
	return nil
}

func (c *ClusterCapacity) SyncWithStore(resourceStore store.ResourceStore) error {
	for _, resource := range resourceStore.Resources() {
		err := c.resourceStore.Replace(resource, resourceStore.List(resource), "0")
		if err != nil {
			return fmt.Errorf("Resource replace error: %v\n", err)
		}
	}
	return nil
}

func (c *ClusterCapacity) Bind(binding *v1.Binding, schedulerName string) error {
	// run the pod through strategy
	key := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: binding.Name, Namespace: binding.Namespace},
	}
	pod, exists, err := c.resourceStore.Get(ccapi.Pods, runtime.Object(key))
	if err != nil {
		return fmt.Errorf("Unable to bind: %v", err)
	}
	if !exists {
		return fmt.Errorf("Unable to bind, pod %v not found", pod)
	}
	updatedPod := *pod.(*v1.Pod)
	updatedPod.Spec.NodeName = binding.Target.Name
	updatedPod.Status.Phase = v1.PodRunning

	// TODO(jchaloup): rename Add to Update as this actually updates the scheduled pod
	if err := c.strategy.Add(&updatedPod); err != nil {
		return fmt.Errorf("Unable to recompute new cluster state: %v", err)
	}

	c.status.Pods = append(c.status.Pods, &updatedPod)
	go func() {
		<-c.schedulerConfigs[schedulerName].Recorder.(*record.Recorder).Events
		//fmt.Printf("Scheduling event: %v\n", event)
	}()

	if c.maxSimulated > 0 && c.simulated >= c.maxSimulated {
		c.status.StopReason = fmt.Sprintf("LimitReached: Maximum number of pods simulated: %v", c.maxSimulated)
		c.Close()
		c.stop <- struct{}{}
		return nil
	}

	// all good, create another pod
	if err := c.nextPod(); err != nil {
		if strings.HasPrefix(c.status.StopReason, "NamespaceNotFound") {
			c.Close()
			c.stop <- struct{}{}
			return nil
		}
		return fmt.Errorf("Unable to create next pod to schedule: %v", err)
	}
	return nil
}

func (c *ClusterCapacity) Close() {
	c.closedMux.Lock()
	defer c.closedMux.Unlock()

	if c.closed {
		return
	}

	for _, name := range c.schedulerConfigs {
		close(name.StopEverything)
	}

	c.coreRestClient.Close()
	close(c.informerStopCh)
	c.closed = true
}

func (c *ClusterCapacity) Update(pod *v1.Pod, podCondition *v1.PodCondition, schedulerName string) error {
	stop := podCondition.Type == v1.PodScheduled && podCondition.Status == v1.ConditionFalse && podCondition.Reason == "Unschedulable"
	if stop {
		c.status.StopReason = fmt.Sprintf("%v: %v", podCondition.Reason, podCondition.Message)
		c.Close()
		// The Update function can be run more than once before any corresponding
		// scheduler is closed. The behaviour is implementation specific
		c.stopMux.Lock()
		defer c.stopMux.Unlock()
		c.stopped = true
		c.stop <- struct{}{}
	}
	return nil
}

func (c *ClusterCapacity) nextPod() error {
	cloner := conversion.NewCloner()
	pod := v1.Pod{}
	if err := v1.DeepCopy_v1_Pod(c.simulatedPod, &pod, cloner); err != nil {
		return err
	}

	// reset any node designation set
	pod.Spec.NodeName = ""
	// use simulated pod name with an index to construct the name
	pod.ObjectMeta.Name = fmt.Sprintf("%v-%v", c.simulatedPod.Name, c.simulated)

	// Check the pod's namespace exists
	_, err := c.externalkubeclient.Core().Namespaces().Get(pod.ObjectMeta.Namespace, metav1.GetOptions{})
	if err != nil {
		c.status.StopReason = fmt.Sprintf("NamespaceNotFound: %v", err)
		return fmt.Errorf("Pod's namespace %v not found: %v", c.simulatedPod.ObjectMeta.Namespace, err)
	}

	c.simulated++
	c.lastSimulatedPod = &pod

	return c.resourceStore.Add(ccapi.Pods, runtime.Object(&pod))
}

func (c *ClusterCapacity) Run() error {
	c.informerFactory.Start(c.informerStopCh)
	// TODO(jchaloup): remove all pods that are not scheduled yet
	for _, scheduler := range c.schedulers {
		scheduler.Run()
	}
	// wait some time before at least nodes are populated
	// TODO(jchaloup); find a better way how to do this or at least decrease it to <100ms
	time.Sleep(100 * time.Millisecond)
	// create the first simulated pod
	err := c.nextPod()
	if err != nil {
		c.Close()
		close(c.stop)
		return fmt.Errorf("Unable to create next pod to schedule: %v", err)
	}
	<-c.stop
	close(c.stop)

	return nil
}

type localBinderPodConditionUpdater struct {
	SchedulerName string
	C             *ClusterCapacity
}

func (b *localBinderPodConditionUpdater) Bind(binding *v1.Binding) error {
	return b.C.Bind(binding, b.SchedulerName)
}

func (b *localBinderPodConditionUpdater) Update(pod *v1.Pod, podCondition *v1.PodCondition) error {
	return b.C.Update(pod, podCondition, b.SchedulerName)
}

func (c *ClusterCapacity) createSchedulerConfig(s *soptions.SchedulerServer) (*scheduler.Config, error) {
	// TODO improve this
	if c.informerFactory == nil {
		c.informerFactory = einformers.NewSharedInformerFactory(c.externalkubeclient, 0)
	}

	fakeClient := fake.NewSimpleClientset()
	fakeInformerFactory := einformers.NewSharedInformerFactory(fakeClient, 0)
	configFactory := factory.NewConfigFactory(s.SchedulerName,
		c.externalkubeclient,
		c.informerFactory.Core().V1().Nodes(),
		c.informerFactory.Core().V1().PersistentVolumes(),
		c.informerFactory.Core().V1().PersistentVolumeClaims(),
		c.informerFactory.Core().V1().ReplicationControllers(),
		c.informerFactory.Extensions().V1beta1().ReplicaSets(),
		fakeInformerFactory.Apps().V1beta1().StatefulSets(),
		c.informerFactory.Core().V1().Services(),
		s.HardPodAffinitySymmetricWeight)
	config, err := createConfig(s, configFactory)

	if err != nil {
		return nil, fmt.Errorf("Failed to create scheduler configuration: %v", err)
	}

	// Collect scheduler succesfully/failed scheduled pod
	config.Recorder = record.NewRecorder(10)
	// Replace the binder with simulator pod counter
	lbpcu := &localBinderPodConditionUpdater{
		SchedulerName: s.SchedulerName,
		C:             c,
	}
	config.Binder = lbpcu
	config.PodConditionUpdater = lbpcu
	// pending merge of https://github.com/kubernetes/kubernetes/pull/44115
	// we wrap how error handling is done to avoid extraneous logging
	errorFn := config.Error
	wrappedErrorFn := func(pod *v1.Pod, err error) {
		if _, ok := err.(*core.FitError); !ok {
			errorFn(pod, err)
		}
	}
	config.Error = wrappedErrorFn
	return config, nil
}

func (c *ClusterCapacity) AddScheduler(s *soptions.SchedulerServer) error {
	config, err := c.createSchedulerConfig(s)
	if err != nil {
		return err
	}

	c.schedulers[s.SchedulerName] = scheduler.New(config)
	c.schedulerConfigs[s.SchedulerName] = config
	return nil
}

func createConfig(s *soptions.SchedulerServer, configFactory scheduler.Configurator) (*scheduler.Config, error) {
	if _, err := os.Stat(s.PolicyConfigFile); err == nil {
		var (
			policy     schedulerapi.Policy
			configData []byte
		)
		configData, err := ioutil.ReadFile(s.PolicyConfigFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read policy config: %v", err)
		}
		if err := runtime.DecodeInto(latestschedulerapi.Codec, configData, &policy); err != nil {
			return nil, fmt.Errorf("invalid configuration: %v", err)
		}
		return configFactory.CreateFromConfig(policy)
	}

	// if the config file isn't provided, use the specified (or default) provider
	return configFactory.CreateFromProvider(s.AlgorithmProvider)
}

// Create new cluster capacity analysis
// The analysis is completely independent of apiserver so no need
// for kubeconfig nor for apiserver url
func New(s *soptions.SchedulerServer, simulatedPod *v1.Pod, maxPods int) (*ClusterCapacity, error) {
	resourceStore := store.NewResourceStore()
	restClient := external.NewRESTClient(resourceStore, "core")

	cc := &ClusterCapacity{
		resourceStore:      resourceStore,
		strategy:           strategy.NewPredictiveStrategy(resourceStore),
		externalkubeclient: externalclientset.New(restClient),
		simulatedPod:       simulatedPod,
		simulated:          0,
		maxSimulated:       maxPods,
		coreRestClient:     restClient,
	}

	for _, resource := range resourceStore.Resources() {
		// The resource variable would be shared among all [Add|Update|Delete]Func functions
		// and resource would be set to the last item in resources list.
		// Thus, it needs to be stored to a local variable in each iteration.
		rt := resource
		resourceStore.RegisterEventHandler(rt, cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				restClient.EmitObjectWatchEvent(rt, watch.Added, obj.(runtime.Object))
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				restClient.EmitObjectWatchEvent(rt, watch.Modified, newObj.(runtime.Object))
			},
			DeleteFunc: func(obj interface{}) {
				restClient.EmitObjectWatchEvent(rt, watch.Deleted, obj.(runtime.Object))
			},
		})
	}

	cc.schedulers = make(map[string]*scheduler.Scheduler)
	cc.schedulerConfigs = make(map[string]*scheduler.Config)

	// read the default scheduler name from configuration
	config, err := cc.createSchedulerConfig(s)
	if err != nil {
		return nil, fmt.Errorf("Unable to create cluster capacity analyzer: %v", err)
	}

	cc.schedulers[s.SchedulerName] = scheduler.New(config)
	cc.schedulerConfigs[s.SchedulerName] = config
	cc.defaultScheduler = s.SchedulerName

	cc.stop = make(chan struct{})
	cc.informerStopCh = make(chan struct{})
	return cc, nil
}
