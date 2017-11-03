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

package controller

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	osb "github.com/pmorie/go-open-service-broker-client/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	servicecatalogclientset "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1beta1"
	informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions/servicecatalog/v1beta1"
	listers "github.com/kubernetes-incubator/service-catalog/pkg/client/listers_generated/servicecatalog/v1beta1"
	pretty "github.com/kubernetes-incubator/service-catalog/pkg/pretty"
)

const (
	// maxRetries is the number of times a resource add/update will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the times
	// a resource is going to be requeued:
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15
	//
	pollingStartInterval      = 1 * time.Second
	pollingMaxBackoffDuration = 1 * time.Hour

	// ContextProfilePlatformKubernetes is the platform name sent in the OSB
	// ContextProfile for requests coming from Kubernetes.
	ContextProfilePlatformKubernetes string = "kubernetes"
)

// NewController returns a new Open Service Broker catalog controller.
func NewController(
	kubeClient kubernetes.Interface,
	serviceCatalogClient servicecatalogclientset.ServicecatalogV1beta1Interface,
	brokerInformer informers.ClusterServiceBrokerInformer,
	clusterServiceClassInformer informers.ClusterServiceClassInformer,
	instanceInformer informers.ServiceInstanceInformer,
	bindingInformer informers.ServiceBindingInformer,
	clusterServicePlanInformer informers.ClusterServicePlanInformer,
	brokerClientCreateFunc osb.CreateFunc,
	brokerRelistInterval time.Duration,
	osbAPIPreferredVersion string,
	recorder record.EventRecorder,
	reconciliationRetryDuration time.Duration,
) (Controller, error) {
	controller := &controller{
		kubeClient:                  kubeClient,
		serviceCatalogClient:        serviceCatalogClient,
		brokerClientCreateFunc:      brokerClientCreateFunc,
		brokerRelistInterval:        brokerRelistInterval,
		OSBAPIPreferredVersion:      osbAPIPreferredVersion,
		recorder:                    recorder,
		reconciliationRetryDuration: reconciliationRetryDuration,
		brokerQueue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "service-broker"),
		serviceClassQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "service-class"),
		servicePlanQueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "service-plan"),
		instanceQueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "service-instance"),
		bindingQueue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "service-binding"),
		pollingQueue:                workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(pollingStartInterval, pollingMaxBackoffDuration), "poller"),
	}

	controller.brokerLister = brokerInformer.Lister()
	brokerInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.brokerAdd,
		UpdateFunc: controller.brokerUpdate,
		DeleteFunc: controller.brokerDelete,
	})

	controller.serviceClassLister = clusterServiceClassInformer.Lister()
	clusterServiceClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.serviceClassAdd,
		UpdateFunc: controller.serviceClassUpdate,
		DeleteFunc: controller.serviceClassDelete,
	})

	controller.servicePlanLister = clusterServicePlanInformer.Lister()
	clusterServicePlanInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.servicePlanAdd,
		UpdateFunc: controller.servicePlanUpdate,
		DeleteFunc: controller.servicePlanDelete,
	})

	controller.instanceLister = instanceInformer.Lister()
	instanceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.instanceAdd,
		UpdateFunc: controller.instanceUpdate,
		DeleteFunc: controller.instanceDelete,
	})

	controller.bindingLister = bindingInformer.Lister()
	bindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.bindingAdd,
		UpdateFunc: controller.bindingUpdate,
		DeleteFunc: controller.bindingDelete,
	})

	return controller, nil
}

// Controller describes a controller that backs the service catalog API for
// Open Service Broker compliant Brokers.
type Controller interface {
	// Run runs the controller until the given stop channel can be read from.
	// workers specifies the number of goroutines, per resource, processing work
	// from the resource workqueues
	Run(workers int, stopCh <-chan struct{})
}

// controller is a concrete Controller.
type controller struct {
	kubeClient                  kubernetes.Interface
	serviceCatalogClient        servicecatalogclientset.ServicecatalogV1beta1Interface
	brokerClientCreateFunc      osb.CreateFunc
	brokerLister                listers.ClusterServiceBrokerLister
	serviceClassLister          listers.ClusterServiceClassLister
	instanceLister              listers.ServiceInstanceLister
	bindingLister               listers.ServiceBindingLister
	servicePlanLister           listers.ClusterServicePlanLister
	brokerRelistInterval        time.Duration
	OSBAPIPreferredVersion      string
	recorder                    record.EventRecorder
	reconciliationRetryDuration time.Duration
	brokerQueue                 workqueue.RateLimitingInterface
	serviceClassQueue           workqueue.RateLimitingInterface
	servicePlanQueue            workqueue.RateLimitingInterface
	instanceQueue               workqueue.RateLimitingInterface
	bindingQueue                workqueue.RateLimitingInterface
	// pollingQueue is separate from instanceQueue because we want
	// it to have different backoff / timeout characteristics from
	//  a reconciling of an instance.
	// TODO(vaikas): get rid of two queues per instance.
	pollingQueue workqueue.RateLimitingInterface
}

// Run runs the controller until the given stop channel can be read from.
func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtimeutil.HandleCrash()

	glog.Info("Starting service-catalog controller")

	var waitGroup sync.WaitGroup

	for i := 0; i < workers; i++ {
		createWorker(c.brokerQueue, "ClusterServiceBroker", maxRetries, true, c.reconcileClusterServiceBrokerKey, stopCh, &waitGroup)
		createWorker(c.serviceClassQueue, "ClusterServiceClass", maxRetries, true, c.reconcileClusterServiceClassKey, stopCh, &waitGroup)
		createWorker(c.servicePlanQueue, "ClusterServicePlan", maxRetries, true, c.reconcileClusterServicePlanKey, stopCh, &waitGroup)
		createWorker(c.instanceQueue, "ServiceInstance", maxRetries, true, c.reconcileServiceInstanceKey, stopCh, &waitGroup)
		createWorker(c.bindingQueue, "ServiceBinding", maxRetries, true, c.reconcileServiceBindingKey, stopCh, &waitGroup)
		createWorker(c.pollingQueue, "Poller", maxRetries, false, c.requeueServiceInstanceForPoll, stopCh, &waitGroup)
	}

	<-stopCh
	glog.Info("Shutting down service-catalog controller")

	c.brokerQueue.ShutDown()
	c.serviceClassQueue.ShutDown()
	c.servicePlanQueue.ShutDown()
	c.instanceQueue.ShutDown()
	c.bindingQueue.ShutDown()
	c.pollingQueue.ShutDown()

	waitGroup.Wait()
}

// createWorker creates and runs a worker thread that just processes items in the
// specified queue. The worker will run until stopCh is closed. The worker will be
// added to the wait group when started and marked done when finished.
func createWorker(queue workqueue.RateLimitingInterface, resourceType string, maxRetries int, forgetAfterSuccess bool, reconciler func(key string) error, stopCh <-chan struct{}, waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)
	go func() {
		wait.Until(worker(queue, resourceType, maxRetries, forgetAfterSuccess, reconciler), time.Second, stopCh)
		waitGroup.Done()
	}()
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// If reconciler returns an error, requeue the item up to maxRetries before giving up.
// It enforces that the reconciler is never invoked concurrently with the same key.
// If forgetAfterSuccess is true, it will cause the queue to forget the item should reconciliation
// have no error.
func worker(queue workqueue.RateLimitingInterface, resourceType string, maxRetries int, forgetAfterSuccess bool, reconciler func(key string) error) func() {
	return func() {
		exit := false
		for !exit {
			exit = func() bool {
				key, quit := queue.Get()
				if quit {
					return true
				}
				defer queue.Done(key)

				err := reconciler(key.(string))
				if err == nil {
					if forgetAfterSuccess {
						queue.Forget(key)
					}
					return false
				}

				if queue.NumRequeues(key) < maxRetries {
					glog.V(4).Infof("Error syncing %s %v: %v", resourceType, key, err)
					queue.AddRateLimited(key)
					return false
				}

				glog.V(4).Infof("Dropping %s %q out of the queue: %v", resourceType, key, err)
				queue.Forget(key)
				return false
			}()
		}
	}
}

// getClusterServiceClassPlanAndClusterServiceBroker is a sequence of operations that's done in couple of
// places so this method fetches the Service Class, Service Plan and creates
// a brokerClient to use for that method given an ServiceInstance.
// The ClusterServicePlan returned will be nil if the ClusterServicePlanRef
// is nil. This will happen when deleting a ServiceInstance that previously
// had an update to a non-existent plan.
func (c *controller) getClusterServiceClassPlanAndClusterServiceBroker(instance *v1beta1.ServiceInstance) (*v1beta1.ClusterServiceClass, *v1beta1.ClusterServicePlan, string, osb.Client, error) {
	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)
	serviceClass, err := c.serviceClassLister.Get(instance.Spec.ClusterServiceClassRef.Name)
	if err != nil {
		s := fmt.Sprintf(
			"References a non-existent ClusterServiceClass (K8S: %q ExternalName: %q)",
			instance.Spec.ClusterServiceClassRef.Name, instance.Spec.ClusterServiceClassExternalName,
		)
		glog.Info(pcb.Message(s))
		c.updateServiceInstanceCondition(
			instance,
			v1beta1.ServiceInstanceConditionReady,
			v1beta1.ConditionFalse,
			errorNonexistentClusterServiceClassReason,
			"The instance references a ClusterServiceClass that does not exist. "+s,
		)
		c.recorder.Event(instance, corev1.EventTypeWarning, errorNonexistentClusterServiceClassReason, s)
		return nil, nil, "", nil, err
	}

	var servicePlan *v1beta1.ClusterServicePlan
	if instance.Spec.ClusterServicePlanRef != nil {
		var err error
		servicePlan, err = c.servicePlanLister.Get(instance.Spec.ClusterServicePlanRef.Name)
		if nil != err {
			s := fmt.Sprintf(
				"References a non-existent ClusterServicePlan (K8S: %q ExternalName: %q) on ClusterServiceClass (K8S: %q ExternalName: %q)",
				instance.Spec.ClusterServicePlanName, instance.Spec.ClusterServicePlanExternalName, serviceClass.Name, serviceClass.Spec.ExternalName,
			)
			glog.Warning(pcb.Message(s))
			c.updateServiceInstanceCondition(
				instance,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				errorNonexistentClusterServicePlanReason,
				"The instance references a ClusterServicePlan that does not exist. "+s,
			)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorNonexistentClusterServicePlanReason, s)
			return nil, nil, "", nil, fmt.Errorf(s)
		}
	}

	broker, err := c.brokerLister.Get(serviceClass.Spec.ClusterServiceBrokerName)
	if err != nil {
		s := fmt.Sprintf("References a non-existent broker %q", serviceClass.Spec.ClusterServiceBrokerName)
		glog.Warning(pcb.Message(s))
		c.updateServiceInstanceCondition(
			instance,
			v1beta1.ServiceInstanceConditionReady,
			v1beta1.ConditionFalse,
			errorNonexistentClusterServiceBrokerReason,
			"The instance references a ClusterServiceBroker that does not exist. "+s,
		)
		c.recorder.Event(instance, corev1.EventTypeWarning, errorNonexistentClusterServiceBrokerReason, s)
		return nil, nil, "", nil, err
	}

	authConfig, err := getAuthCredentialsFromClusterServiceBroker(c.kubeClient, broker)
	if err != nil {
		s := fmt.Sprintf("Error getting broker auth credentials for broker %q: %s", broker.Name, err)
		glog.Info(pcb.Message(s))
		c.updateServiceInstanceCondition(
			instance,
			v1beta1.ServiceInstanceConditionReady,
			v1beta1.ConditionFalse,
			errorAuthCredentialsReason,
			"Error getting auth credentials. "+s,
		)
		c.recorder.Event(instance, corev1.EventTypeWarning, errorAuthCredentialsReason, s)
		return nil, nil, "", nil, err
	}

	clientConfig := NewClientConfigurationForBroker(broker, authConfig)

	s := fmt.Sprintf("Creating client for ClusterServiceBroker %v, URL: %v", broker.Name, broker.Spec.URL)
	glog.V(4).Info(pcb.Message(s))
	brokerClient, err := c.brokerClientCreateFunc(clientConfig)
	if err != nil {
		return nil, nil, "", nil, err
	}

	return serviceClass, servicePlan, broker.Name, brokerClient, nil
}

// getClusterServiceClassPlanAndClusterServiceBrokerForServiceBinding is a sequence of operations that's
// done to validate service plan, service class exist, and handles creating
// a brokerclient to use for a given ServiceInstance.
// Sets ClusterServiceClassRef and/or ClusterServicePlanRef if they haven't been already set.
func (c *controller) getClusterServiceClassPlanAndClusterServiceBrokerForServiceBinding(instance *v1beta1.ServiceInstance, binding *v1beta1.ServiceBinding) (*v1beta1.ClusterServiceClass, *v1beta1.ClusterServicePlan, string, osb.Client, error) {
	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)
	serviceClass, err := c.serviceClassLister.Get(instance.Spec.ClusterServiceClassRef.Name)
	if err != nil {
		s := fmt.Sprintf(
			"References a non-existent ClusterServiceClass (K8S: %q ExternalName: %q)",
			instance.Spec.ClusterServiceClassRef.Name, instance.Spec.ClusterServiceClassExternalName,
		)
		glog.Warning(pcb.Message(s))
		c.updateServiceBindingCondition(
			binding,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionFalse,
			errorNonexistentClusterServiceClassReason,
			"The binding references a ClusterServiceClass that does not exist. "+s,
		)
		c.recorder.Event(binding, corev1.EventTypeWarning, errorNonexistentClusterServiceClassMessage, s)
		return nil, nil, "", nil, err
	}

	servicePlan, err := c.servicePlanLister.Get(instance.Spec.ClusterServicePlanRef.Name)
	if nil != err {
		s := fmt.Sprintf(
			"References a non-existent ClusterServicePlan (K8S: %q ExternalName: %q) on ClusterServiceClass (K8S: %q ExternalName: %q)",
			instance.Spec.ClusterServicePlanName, instance.Spec.ClusterServicePlanExternalName, serviceClass.Name, serviceClass.Spec.ExternalName,
		)
		glog.Warning(pcb.Message(s))
		c.updateServiceBindingCondition(
			binding,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionFalse,
			errorNonexistentClusterServicePlanReason,
			"The ServiceBinding references an ServiceInstance which references ClusterServicePlan that does not exist. "+s,
		)
		c.recorder.Event(binding, corev1.EventTypeWarning, errorNonexistentClusterServicePlanReason, s)
		return nil, nil, "", nil, fmt.Errorf(s)
	}

	broker, err := c.brokerLister.Get(serviceClass.Spec.ClusterServiceBrokerName)
	if err != nil {
		s := fmt.Sprintf("References a non-existent ClusterServiceBroker %q", serviceClass.Spec.ClusterServiceBrokerName)
		glog.Warning(pcb.Message(s))
		c.updateServiceBindingCondition(
			binding,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionFalse,
			errorNonexistentClusterServiceBrokerReason,
			"The binding references a ClusterServiceBroker that does not exist. "+s,
		)
		c.recorder.Event(binding, corev1.EventTypeWarning, errorNonexistentClusterServiceBrokerReason, s)
		return nil, nil, "", nil, err
	}

	authConfig, err := getAuthCredentialsFromClusterServiceBroker(c.kubeClient, broker)
	if err != nil {
		s := fmt.Sprintf("Error getting broker auth credentials for broker %q: %s", broker.Name, err)
		glog.Warning(pcb.Message(s))
		c.updateServiceBindingCondition(
			binding,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionFalse,
			errorAuthCredentialsReason,
			"Error getting auth credentials. "+s,
		)
		c.recorder.Event(binding, corev1.EventTypeWarning, errorAuthCredentialsReason, s)
		return nil, nil, "", nil, err
	}

	clientConfig := NewClientConfigurationForBroker(broker, authConfig)

	glog.V(4).Infof("Creating client for ClusterServiceBroker %v, URL: %v", broker.Name, broker.Spec.URL)
	brokerClient, err := c.brokerClientCreateFunc(clientConfig)
	if err != nil {
		return nil, nil, "", nil, err
	}

	return serviceClass, servicePlan, broker.Name, brokerClient, nil
}

// Broker utility methods - move?
// getAuthCredentialsFromClusterServiceBroker returns the auth credentials, if any, or
// returns an error. If the AuthInfo field is nil, empty values are
// returned.
func getAuthCredentialsFromClusterServiceBroker(client kubernetes.Interface, broker *v1beta1.ClusterServiceBroker) (*osb.AuthConfig, error) {
	if broker.Spec.AuthInfo == nil {
		return nil, nil
	}

	authInfo := broker.Spec.AuthInfo
	if authInfo.Basic != nil {
		secretRef := authInfo.Basic.SecretRef
		secret, err := client.CoreV1().Secrets(secretRef.Namespace).Get(secretRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		basicAuthConfig, err := getBasicAuthConfig(secret)
		if err != nil {
			return nil, err
		}
		return &osb.AuthConfig{
			BasicAuthConfig: basicAuthConfig,
		}, nil
	} else if authInfo.Bearer != nil {
		secretRef := authInfo.Bearer.SecretRef
		secret, err := client.CoreV1().Secrets(secretRef.Namespace).Get(secretRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		bearerConfig, err := getBearerConfig(secret)
		if err != nil {
			return nil, err
		}
		return &osb.AuthConfig{
			BearerConfig: bearerConfig,
		}, nil
	}
	return nil, fmt.Errorf("empty auth info or unsupported auth mode: %s", authInfo)
}

func getBasicAuthConfig(secret *corev1.Secret) (*osb.BasicAuthConfig, error) {
	usernameBytes, ok := secret.Data["username"]
	if !ok {
		return nil, fmt.Errorf("auth secret didn't contain username")
	}

	passwordBytes, ok := secret.Data["password"]
	if !ok {
		return nil, fmt.Errorf("auth secret didn't contain password")
	}

	return &osb.BasicAuthConfig{
		Username: string(usernameBytes),
		Password: string(passwordBytes),
	}, nil
}

func getBearerConfig(secret *corev1.Secret) (*osb.BearerConfig, error) {
	tokenBytes, ok := secret.Data["token"]
	if !ok {
		return nil, fmt.Errorf("auth secret didn't contain token")
	}

	return &osb.BearerConfig{
		Token: string(tokenBytes),
	}, nil
}

// convertCatalog converts a service broker catalog into an array of
// ClusterServiceClasses and an array of ClusterServicePlans.  The ClusterServiceClasses and
// ClusterServicePlans returned by this method are named in K8S with the OSB ID.
func convertCatalog(in *osb.CatalogResponse) ([]*v1beta1.ClusterServiceClass, []*v1beta1.ClusterServicePlan, error) {
	serviceClasses := make([]*v1beta1.ClusterServiceClass, len(in.Services))
	servicePlans := []*v1beta1.ClusterServicePlan{}
	for i, svc := range in.Services {
		serviceClasses[i] = &v1beta1.ClusterServiceClass{
			Spec: v1beta1.ClusterServiceClassSpec{
				Bindable:      svc.Bindable,
				PlanUpdatable: (svc.PlanUpdatable != nil && *svc.PlanUpdatable),
				ExternalID:    svc.ID,
				ExternalName:  svc.Name,
				Tags:          svc.Tags,
				Description:   svc.Description,
				Requires:      svc.Requires,
			},
		}

		if svc.Metadata != nil {
			metadata, err := json.Marshal(svc.Metadata)
			if err != nil {
				err = fmt.Errorf("Failed to marshal metadata\n%+v\n %v", svc.Metadata, err)
				glog.Error(err)
				return nil, nil, err
			}
			serviceClasses[i].Spec.ExternalMetadata = &runtime.RawExtension{Raw: metadata}
		}

		serviceClasses[i].SetName(svc.ID)

		// set up the plans using the ClusterServiceClass Name
		plans, err := convertClusterServicePlans(svc.Plans, serviceClasses[i].Name)
		if err != nil {
			return nil, nil, err
		}
		servicePlans = append(servicePlans, plans...)
	}
	return serviceClasses, servicePlans, nil
}

func convertClusterServicePlans(plans []osb.Plan, serviceClassID string) ([]*v1beta1.ClusterServicePlan, error) {
	if 0 == len(plans) {
		return nil, fmt.Errorf("ClusterServiceClass (K8S: %q) must have at least one plan", serviceClassID)
	}
	servicePlans := make([]*v1beta1.ClusterServicePlan, len(plans))
	for i, plan := range plans {
		servicePlans[i] = &v1beta1.ClusterServicePlan{
			Spec: v1beta1.ClusterServicePlanSpec{
				ExternalName:           plan.Name,
				ExternalID:             plan.ID,
				Free:                   plan.Free != nil && *plan.Free,
				Description:            plan.Description,
				ClusterServiceClassRef: v1beta1.ClusterObjectReference{Name: serviceClassID},
			},
		}
		servicePlans[i].SetName(plan.ID)

		if plan.Bindable != nil {
			b := *plan.Bindable
			servicePlans[i].Spec.Bindable = &b
		}

		if plan.Metadata != nil {
			metadata, err := json.Marshal(plan.Metadata)
			if err != nil {
				err = fmt.Errorf("Failed to marshal metadata\n%+v\n %v", plan.Metadata, err)
				glog.Error(err)
				return nil, err
			}
			servicePlans[i].Spec.ExternalMetadata = &runtime.RawExtension{Raw: metadata}
		}

		if schemas := plan.ParameterSchemas; schemas != nil {
			if instanceSchemas := schemas.ServiceInstances; instanceSchemas != nil {
				if instanceCreateSchema := instanceSchemas.Create; instanceCreateSchema != nil && instanceCreateSchema.Parameters != nil {
					schema, err := json.Marshal(instanceCreateSchema.Parameters)
					if err != nil {
						err = fmt.Errorf("Failed to marshal instance create schema \n%+v\n %v", instanceCreateSchema.Parameters, err)
						glog.Error(err)
						return nil, err
					}
					servicePlans[i].Spec.ServiceInstanceCreateParameterSchema = &runtime.RawExtension{Raw: schema}
				}
				if instanceUpdateSchema := instanceSchemas.Update; instanceUpdateSchema != nil && instanceUpdateSchema.Parameters != nil {
					schema, err := json.Marshal(instanceUpdateSchema.Parameters)
					if err != nil {
						err = fmt.Errorf("Failed to marshal instance update schema \n%+v\n %v", instanceUpdateSchema.Parameters, err)
						glog.Error(err)
						return nil, err
					}
					servicePlans[i].Spec.ServiceInstanceUpdateParameterSchema = &runtime.RawExtension{Raw: schema}
				}
			}
			if bindingSchemas := schemas.ServiceBindings; bindingSchemas != nil {
				if bindingCreateSchema := bindingSchemas.Create; bindingCreateSchema != nil && bindingCreateSchema.Parameters != nil {
					schema, err := json.Marshal(bindingCreateSchema.Parameters)
					if err != nil {
						err = fmt.Errorf("Failed to marshal binding create schema \n%+v\n %v", bindingCreateSchema.Parameters, err)
						glog.Error(err)
						return nil, err
					}
					servicePlans[i].Spec.ServiceBindingCreateParameterSchema = &runtime.RawExtension{Raw: schema}
				}
			}
		}

	}
	return servicePlans, nil
}

// isServiceInstanceReady returns whether the given instance has a ready condition
// with status true.
func isServiceInstanceReady(instance *v1beta1.ServiceInstance) bool {
	for _, cond := range instance.Status.Conditions {
		if cond.Type == v1beta1.ServiceInstanceConditionReady {
			return cond.Status == v1beta1.ConditionTrue
		}
	}

	return false
}

// NewClientConfigurationForBroker creates a new ClientConfiguration for connecting
// to the specified Broker
func NewClientConfigurationForBroker(broker *v1beta1.ClusterServiceBroker, authConfig *osb.AuthConfig) *osb.ClientConfiguration {
	clientConfig := osb.DefaultClientConfiguration()
	clientConfig.Name = broker.Name
	clientConfig.URL = broker.Spec.URL
	clientConfig.AuthConfig = authConfig
	clientConfig.EnableAlphaFeatures = true
	clientConfig.Insecure = broker.Spec.InsecureSkipTLSVerify
	clientConfig.CAData = broker.Spec.CABundle
	return clientConfig
}
