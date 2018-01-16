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
	"sync"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/informers"
	storageinformers "k8s.io/client-go/informers/storage/v1"
	externalclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	//"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/features"
	sapps "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app"
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

const (
	podProvisioner = "cc.kubernetes.io/provisioned-by"
)

type ClusterCapacity struct {
	// caches modified by emulation strategy
	resourceStore store.ResourceStore

	// emulation strategy
	strategy strategy.Strategy

	externalkubeclient *externalclientset.Clientset

	informerFactory informers.SharedInformerFactory

	// schedulers
	schedulers           map[string]*scheduler.Scheduler
	schedulerConfigs     map[string]*scheduler.Config
	defaultSchedulerName string

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
		listWatcher := cache.NewListWatchFromClient(client.Core().RESTClient(), resource.String(), metav1.NamespaceAll, fields.ParseSelectorOrDie(""))

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
	}()

	if c.maxSimulated > 0 && c.simulated >= c.maxSimulated {
		c.status.StopReason = fmt.Sprintf("LimitReached: Maximum number of pods simulated: %v", c.maxSimulated)
		c.Close()
		c.stop <- struct{}{}
		return nil
	}

	// all good, create another pod
	if err := c.nextPod(); err != nil {
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

	close(c.informerStopCh)
	c.closed = true
}

func (c *ClusterCapacity) Update(pod *v1.Pod, podCondition *v1.PodCondition, schedulerName string) error {
	stop := podCondition.Type == v1.PodScheduled && podCondition.Status == v1.ConditionFalse && podCondition.Reason == "Unschedulable"

	// Only for pending pods provisioned by cluster-capacity
	if stop && metav1.HasAnnotation(pod.ObjectMeta, podProvisioner) {
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
	pod := v1.Pod{}
	pod = *c.simulatedPod.DeepCopy()
	// reset any node designation set
	pod.Spec.NodeName = ""
	// use simulated pod name with an index to construct the name
	pod.ObjectMeta.Name = fmt.Sprintf("%v-%v", c.simulatedPod.Name, c.simulated)

	// Add pod provisioner annotation
	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = map[string]string{}
	}

	// Stores the scheduler name
	pod.ObjectMeta.Annotations[podProvisioner] = c.defaultSchedulerName

	c.simulated++
	c.lastSimulatedPod = &pod

	return c.resourceStore.Add(ccapi.Pods, runtime.Object(&pod))
}

func (c *ClusterCapacity) Run() error {
	// Start all informers.
	c.informerFactory.Start(c.informerStopCh)
	c.informerFactory.WaitForCacheSync(c.informerStopCh)

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

func (c *ClusterCapacity) createScheduler(s *sapps.SchedulerServer) (*scheduler.Scheduler, error) {
	c.informerFactory = s.InformerFactory
	s.Recorder = record.NewRecorder(10)

	schedulerConfig, err := SchedulerConfigLocal(s)
	if err != nil {
		return nil, err
	}

	// Replace the binder with simulator pod counter
	lbpcu := &localBinderPodConditionUpdater{
		SchedulerName: s.SchedulerName,
		C:             c,
	}
	schedulerConfig.Binder = lbpcu
	schedulerConfig.PodConditionUpdater = lbpcu
	// pending merge of https://github.com/kubernetes/kubernetes/pull/44115
	// we wrap how error handling is done to avoid extraneous logging
	errorFn := schedulerConfig.Error
	wrappedErrorFn := func(pod *v1.Pod, err error) {
		if _, ok := err.(*core.FitError); !ok {
			errorFn(pod, err)
		}
	}
	schedulerConfig.Error = wrappedErrorFn
	// Create the scheduler.
	scheduler := scheduler.NewFromConfig(schedulerConfig)

	return scheduler, nil
}

// TODO(avesh): enable when support for multiple schedulers is added.
/*func (c *ClusterCapacity) AddScheduler(s *sapps.SchedulerServer) error {
	scheduler, err := c.createScheduler(s)
	if err != nil {
		return err
	}

	c.schedulers[s.SchedulerName] = scheduler
	c.schedulerConfigs[s.SchedulerName] = scheduler.Config()
	return nil
}*/

// Create new cluster capacity analysis
// The analysis is completely independent of apiserver so no need
// for kubeconfig nor for apiserver url
func New(schedServer *sapps.SchedulerServer, simulatedPod *v1.Pod, maxPods int) (*ClusterCapacity, error) {
	resourceStore := store.NewResourceStore()
	restClient := external.NewRESTClient(resourceStore, "core")

	cc := &ClusterCapacity{
		resourceStore:      resourceStore,
		strategy:           strategy.NewPredictiveStrategy(resourceStore),
		externalkubeclient: externalclientset.New(restClient),
		simulatedPod:       simulatedPod,
		simulated:          0,
		maxSimulated:       maxPods,
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

	// Replace InformerFactory
	schedServer.InformerFactory = informers.NewSharedInformerFactory(cc.externalkubeclient, 0)
	schedServer.Client = cc.externalkubeclient

	cc.schedulers = make(map[string]*scheduler.Scheduler)
	cc.schedulerConfigs = make(map[string]*scheduler.Config)

	scheduler, err := cc.createScheduler(schedServer)
	if err != nil {
		return nil, err
	}

	cc.schedulers[schedServer.SchedulerName] = scheduler
	cc.schedulerConfigs[schedServer.SchedulerName] = scheduler.Config()
	cc.defaultSchedulerName = schedServer.SchedulerName
	cc.stop = make(chan struct{})
	cc.informerStopCh = make(chan struct{})
	return cc, nil
}

// SchedulerConfig creates the scheduler configuration.
func SchedulerConfigLocal(s *sapps.SchedulerServer) (*scheduler.Config, error) {
	var storageClassInformer storageinformers.StorageClassInformer
	if utilfeature.DefaultFeatureGate.Enabled(features.VolumeScheduling) {
		storageClassInformer = s.InformerFactory.Storage().V1().StorageClasses()
	}

	fakeClient := fake.NewSimpleClientset()
	fakeInformerFactory := informers.NewSharedInformerFactory(fakeClient, 0)

	// Set up the configurator which can create schedulers from configs.
	configurator := factory.NewConfigFactory(
		s.SchedulerName,
		s.Client,
		s.InformerFactory.Core().V1().Nodes(),
		s.InformerFactory.Core().V1().Pods(),
		s.InformerFactory.Core().V1().PersistentVolumes(),
		s.InformerFactory.Core().V1().PersistentVolumeClaims(),
		fakeInformerFactory.Core().V1().ReplicationControllers(),
		fakeInformerFactory.Extensions().V1beta1().ReplicaSets(),
		fakeInformerFactory.Apps().V1beta1().StatefulSets(),
		s.InformerFactory.Core().V1().Services(),
		fakeInformerFactory.Policy().V1beta1().PodDisruptionBudgets(),
		storageClassInformer,
		s.HardPodAffinitySymmetricWeight,
		utilfeature.DefaultFeatureGate.Enabled(features.EnableEquivalenceClassCache),
	)

	source := s.AlgorithmSource
	var config *scheduler.Config
	switch {
	case source.Provider != nil:
		// Create the config from a named algorithm provider.
		sc, err := configurator.CreateFromProvider(*source.Provider)
		if err != nil {
			return nil, fmt.Errorf("couldn't create scheduler using provider %q: %v", *source.Provider, err)
		}
		config = sc
	case source.Policy != nil:
		// Create the config from a user specified policy source.
		policy := &schedulerapi.Policy{}
		switch {
		case source.Policy.File != nil:
			// Use a policy serialized in a file.
			policyFile := source.Policy.File.Path
			_, err := os.Stat(policyFile)
			if err != nil {
				return nil, fmt.Errorf("missing policy config file %s", policyFile)
			}
			data, err := ioutil.ReadFile(policyFile)
			if err != nil {
				return nil, fmt.Errorf("couldn't read policy config: %v", err)
			}
			err = runtime.DecodeInto(latestschedulerapi.Codec, []byte(data), policy)
			if err != nil {
				return nil, fmt.Errorf("invalid policy: %v", err)
			}
		case source.Policy.ConfigMap != nil:
			// Use a policy serialized in a config map value.
			policyRef := source.Policy.ConfigMap
			policyConfigMap, err := s.Client.CoreV1().ConfigMaps(policyRef.Namespace).Get(policyRef.Name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("couldn't get policy config map %s/%s: %v", policyRef.Namespace, policyRef.Name, err)
			}
			data, found := policyConfigMap.Data[componentconfig.SchedulerPolicyConfigMapKey]
			if !found {
				return nil, fmt.Errorf("missing policy config map value at key %q", componentconfig.SchedulerPolicyConfigMapKey)
			}
			err = runtime.DecodeInto(latestschedulerapi.Codec, []byte(data), policy)
			if err != nil {
				return nil, fmt.Errorf("invalid policy: %v", err)
			}
		}
		sc, err := configurator.CreateFromConfig(*policy)
		if err != nil {
			return nil, fmt.Errorf("couldn't create scheduler from policy: %v", err)
		}
		config = sc
	default:
		return nil, fmt.Errorf("unsupported algorithm source: %v", source)
	}
	// Additional tweaks to the config produced by the configurator.
	config.Recorder = s.Recorder
	return config, nil
}
