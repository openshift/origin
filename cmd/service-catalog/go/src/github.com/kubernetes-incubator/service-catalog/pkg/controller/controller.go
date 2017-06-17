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
	"time"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi"
	servicecatalogclientset "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1alpha1"
	informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions/servicecatalog/v1alpha1"
	listers "github.com/kubernetes-incubator/service-catalog/pkg/client/listers_generated/servicecatalog/v1alpha1"
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
)

// NewController returns a new Open Service Broker catalog controller.
func NewController(
	kubeClient kubernetes.Interface,
	serviceCatalogClient servicecatalogclientset.ServicecatalogV1alpha1Interface,
	brokerInformer informers.BrokerInformer,
	serviceClassInformer informers.ServiceClassInformer,
	instanceInformer informers.InstanceInformer,
	bindingInformer informers.BindingInformer,
	brokerClientCreateFunc brokerapi.CreateFunc,
	brokerRelistInterval time.Duration,
	osbAPIContextProfile bool,
	recorder record.EventRecorder,
) (Controller, error) {
	controller := &controller{
		kubeClient:                kubeClient,
		serviceCatalogClient:      serviceCatalogClient,
		brokerClientCreateFunc:    brokerClientCreateFunc,
		brokerRelistInterval:      brokerRelistInterval,
		enableOSBAPIContextProfle: osbAPIContextProfile,
		recorder:                  recorder,
		brokerQueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "broker"),
		serviceClassQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "service-class"),
		instanceQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "instance"),
		bindingQueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "binding"),
		pollingQueue:              workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(pollingStartInterval, pollingMaxBackoffDuration), "poller"),
	}

	brokerInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.brokerAdd,
		UpdateFunc: controller.brokerUpdate,
		DeleteFunc: controller.brokerDelete,
	})
	controller.brokerLister = brokerInformer.Lister()

	serviceClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.serviceClassAdd,
		UpdateFunc: controller.serviceClassUpdate,
		DeleteFunc: controller.serviceClassDelete,
	})
	controller.serviceClassLister = serviceClassInformer.Lister()

	instanceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.instanceAdd,
		UpdateFunc: controller.instanceUpdate,
		DeleteFunc: controller.instanceDelete,
	})
	controller.instanceLister = instanceInformer.Lister()

	bindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.bindingAdd,
		UpdateFunc: controller.bindingUpdate,
		DeleteFunc: controller.bindingDelete,
	})
	controller.bindingLister = bindingInformer.Lister()

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
	kubeClient                kubernetes.Interface
	serviceCatalogClient      servicecatalogclientset.ServicecatalogV1alpha1Interface
	brokerClientCreateFunc    brokerapi.CreateFunc
	brokerLister              listers.BrokerLister
	serviceClassLister        listers.ServiceClassLister
	instanceLister            listers.InstanceLister
	bindingLister             listers.BindingLister
	brokerRelistInterval      time.Duration
	enableOSBAPIContextProfle bool
	recorder                  record.EventRecorder
	brokerQueue               workqueue.RateLimitingInterface
	serviceClassQueue         workqueue.RateLimitingInterface
	instanceQueue             workqueue.RateLimitingInterface
	bindingQueue              workqueue.RateLimitingInterface
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

	for i := 0; i < workers; i++ {
		go wait.Until(worker(c.brokerQueue, "Broker", maxRetries, c.reconcileBrokerKey), time.Second, stopCh)
		go wait.Until(worker(c.serviceClassQueue, "ServiceClass", maxRetries, c.reconcileServiceClassKey), time.Second, stopCh)
		go wait.Until(worker(c.instanceQueue, "Instance", maxRetries, c.reconcileInstanceKey), time.Second, stopCh)
		go wait.Until(worker(c.bindingQueue, "Binding", maxRetries, c.reconcileBindingKey), time.Second, stopCh)
		go wait.Until(worker(c.pollingQueue, "Poller", maxRetries, c.reconcileInstanceKey), time.Second, stopCh)
	}

	<-stopCh
	glog.Info("Shutting down service-catalog controller")

	c.brokerQueue.ShutDown()
	c.serviceClassQueue.ShutDown()
	c.instanceQueue.ShutDown()
	c.bindingQueue.ShutDown()
	c.pollingQueue.ShutDown()
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// If reconciler returns an error, requeue the item up to maxRetries before giving up.
// It enforces that the reconciler is never invoked concurrently with the same key.
// TODO: Consider allowing the reconciler to return an error that either specifies whether
// this is recoverable or not, rather than always continuing on an error condition. Seems
// like it should be possible to return an error, yet stop any further polling work.
func worker(queue workqueue.RateLimitingInterface, resourceType string, maxRetries int, reconciler func(key string) error) func() {
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
					queue.Forget(key)
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

// getServiceClassPlanAndBroker is a sequence of operations that's done in couple of
// places so this method fetches the Service Class, Service Plan and creates
// a brokerClient to use for that method given an Instance.
func (c *controller) getServiceClassPlanAndBroker(instance *v1alpha1.Instance) (*v1alpha1.ServiceClass, *v1alpha1.ServicePlan, string, brokerapi.BrokerClient, error) {
	serviceClass, err := c.serviceClassLister.Get(instance.Spec.ServiceClassName)
	if err != nil {
		s := fmt.Sprintf("Instance \"%s/%s\" references a non-existent ServiceClass %q", instance.Namespace, instance.Name, instance.Spec.ServiceClassName)
		glog.Info(s)
		c.updateInstanceCondition(
			instance,
			v1alpha1.InstanceConditionReady,
			v1alpha1.ConditionFalse,
			errorNonexistentServiceClassReason,
			"The instance references a ServiceClass that does not exist. "+s,
		)
		c.recorder.Event(instance, api.EventTypeWarning, errorNonexistentServiceClassReason, s)
		return nil, nil, "", nil, err
	}

	servicePlan := findServicePlan(instance.Spec.PlanName, serviceClass.Plans)
	if servicePlan == nil {
		s := fmt.Sprintf("Instance \"%s/%s\" references a non-existent ServicePlan %q on ServiceClass %q", instance.Namespace, instance.Name, instance.Spec.PlanName, serviceClass.Name)
		glog.Warning(s)
		c.updateInstanceCondition(
			instance,
			v1alpha1.InstanceConditionReady,
			v1alpha1.ConditionFalse,
			"ReferencesNonexistentServicePlan",
			"The instance references a ServicePlan that does not exist. "+s,
		)
		c.recorder.Event(instance, api.EventTypeWarning, errorNonexistentServicePlanReason, s)
		return nil, nil, "", nil, fmt.Errorf(s)
	}

	broker, err := c.brokerLister.Get(serviceClass.BrokerName)
	if err != nil {
		s := fmt.Sprintf("Instance \"%s/%s\" references a non-existent broker %q", instance.Namespace, instance.Name, serviceClass.BrokerName)
		glog.Warning(s)
		c.updateInstanceCondition(
			instance,
			v1alpha1.InstanceConditionReady,
			v1alpha1.ConditionFalse,
			errorNonexistentBrokerReason,
			"The instance references a Broker that does not exist. "+s,
		)
		c.recorder.Event(instance, api.EventTypeWarning, errorNonexistentBrokerReason, s)
		return nil, nil, "", nil, err
	}

	username, password, err := getAuthCredentialsFromBroker(c.kubeClient, broker)
	if err != nil {
		s := fmt.Sprintf("Error getting broker auth credentials for broker %q: %s", broker.Name, err)
		glog.Info(s)
		c.updateInstanceCondition(
			instance,
			v1alpha1.InstanceConditionReady,
			v1alpha1.ConditionFalse,
			errorAuthCredentialsReason,
			"Error getting auth credentials. "+s,
		)
		c.recorder.Event(instance, api.EventTypeWarning, errorAuthCredentialsReason, s)
		return nil, nil, "", nil, err
	}

	glog.V(4).Infof("Creating client for Broker %v, URL: %v", broker.Name, broker.Spec.URL)
	brokerClient := c.brokerClientCreateFunc(broker.Name, broker.Spec.URL, username, password)
	return serviceClass, servicePlan, broker.Name, brokerClient, nil
}

// getServiceClassPlanAndBrokerForBinding is a sequence of operations that's
// done to validate service plan, service class exist, and handles creating
// a brokerclient to use for a given Instance.
func (c *controller) getServiceClassPlanAndBrokerForBinding(instance *v1alpha1.Instance, binding *v1alpha1.Binding) (*v1alpha1.ServiceClass, *v1alpha1.ServicePlan, string, brokerapi.BrokerClient, error) {
	serviceClass, err := c.serviceClassLister.Get(instance.Spec.ServiceClassName)
	if err != nil {
		s := fmt.Sprintf("Binding \"%s/%s\" references a non-existent ServiceClass %q", binding.Namespace, binding.Name, instance.Spec.ServiceClassName)
		glog.Warning(s)
		c.updateBindingCondition(
			binding,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			errorNonexistentServiceClassReason,
			"The binding references a ServiceClass that does not exist. "+s,
		)
		c.recorder.Event(binding, api.EventTypeWarning, "ReferencesNonexistentServiceClass", s)
		return nil, nil, "", nil, err
	}

	servicePlan := findServicePlan(instance.Spec.PlanName, serviceClass.Plans)
	if servicePlan == nil {
		s := fmt.Sprintf("Instance \"%s/%s\" references a non-existent ServicePlan %q on ServiceClass %q", instance.Namespace, instance.Name, instance.Spec.PlanName, serviceClass.Name)
		glog.Warning(s)
		c.updateBindingCondition(
			binding,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			errorNonexistentServicePlanReason,
			"The Binding references an Instance which references ServicePlan that does not exist. "+s,
		)
		c.recorder.Event(binding, api.EventTypeWarning, errorNonexistentServicePlanReason, s)
		return nil, nil, "", nil, fmt.Errorf(s)
	}

	broker, err := c.brokerLister.Get(serviceClass.BrokerName)
	if err != nil {
		s := fmt.Sprintf("Binding \"%s/%s\" references a non-existent Broker %q", binding.Namespace, binding.Name, serviceClass.BrokerName)
		glog.Warning(s)
		c.updateBindingCondition(
			binding,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			errorNonexistentBrokerReason,
			"The binding references a Broker that does not exist. "+s,
		)
		c.recorder.Event(binding, api.EventTypeWarning, errorNonexistentBrokerReason, s)
		return nil, nil, "", nil, err
	}

	username, password, err := getAuthCredentialsFromBroker(c.kubeClient, broker)
	if err != nil {
		s := fmt.Sprintf("Error getting broker auth credentials for broker %q: %s", broker.Name, err)
		glog.Warning(s)
		c.updateBindingCondition(
			binding,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			errorAuthCredentialsReason,
			"Error getting auth credentials. "+s,
		)
		c.recorder.Event(binding, api.EventTypeWarning, errorAuthCredentialsReason, s)
		return nil, nil, "", nil, err
	}

	glog.V(4).Infof("Creating client for Broker %v, URL: %v", broker.Name, broker.Spec.URL)
	brokerClient := c.brokerClientCreateFunc(broker.Name, broker.Spec.URL, username, password)
	return serviceClass, servicePlan, broker.Name, brokerClient, nil
}

// Broker utility methods - move?

// getAuthCredentialsFromBroker returns the auth credentials, if any,
// contained in the secret referenced in the Broker's AuthSecret field, or
// returns an error. If the AuthSecret field is nil, empty values are
// returned.
func getAuthCredentialsFromBroker(client kubernetes.Interface, broker *v1alpha1.Broker) (username, password string, err error) {
	// TODO: when we start supporting additional auth schemes, this code will have to accommodate
	// the new schemes
	if broker.Spec.AuthInfo == nil {
		return "", "", nil
	}

	basicAuthSecret := broker.Spec.AuthInfo.BasicAuthSecret

	if basicAuthSecret == nil {
		return "", "", nil
	}

	authSecret, err := client.Core().Secrets(basicAuthSecret.Namespace).Get(basicAuthSecret.Name, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}

	usernameBytes, ok := authSecret.Data["username"]
	if !ok {
		return "", "", fmt.Errorf("auth secret didn't contain username")
	}

	passwordBytes, ok := authSecret.Data["password"]
	if !ok {
		return "", "", fmt.Errorf("auth secret didn't contain password")
	}

	return string(usernameBytes), string(passwordBytes), nil
}

// convertCatalog converts a service broker catalog into an array of ServiceClasses
func convertCatalog(in *brokerapi.Catalog) ([]*v1alpha1.ServiceClass, error) {
	ret := make([]*v1alpha1.ServiceClass, len(in.Services))
	for i, svc := range in.Services {
		plans, err := convertServicePlans(svc.Plans)
		if err != nil {
			return nil, err
		}
		ret[i] = &v1alpha1.ServiceClass{
			Bindable:      svc.Bindable,
			Plans:         plans,
			PlanUpdatable: svc.PlanUpdateable,
			ExternalID:    svc.ID,
			AlphaTags:     svc.Tags,
			Description:   svc.Description,
			AlphaRequires: svc.Requires,
		}

		if svc.Metadata != nil {
			metadata, err := json.Marshal(svc.Metadata)
			if err != nil {
				err = fmt.Errorf("Failed to marshal metadata\n%+v\n %v", svc.Metadata, err)
				glog.Error(err)
				return nil, err
			}
			ret[i].ExternalMetadata = &runtime.RawExtension{Raw: metadata}
		}

		ret[i].SetName(svc.Name)
	}
	return ret, nil
}

func convertServicePlans(plans []brokerapi.ServicePlan) ([]v1alpha1.ServicePlan, error) {
	ret := make([]v1alpha1.ServicePlan, len(plans))
	for i := range plans {
		ret[i] = v1alpha1.ServicePlan{
			Name:        plans[i].Name,
			ExternalID:  plans[i].ID,
			Free:        plans[i].Free,
			Description: plans[i].Description,
		}

		if plans[i].Bindable != nil {
			b := *plans[i].Bindable
			ret[i].Bindable = &b
		}

		if plans[i].Metadata != nil {
			metadata, err := json.Marshal(plans[i].Metadata)
			if err != nil {
				err = fmt.Errorf("Failed to marshal metadata\n%+v\n %v", plans[i].Metadata, err)
				glog.Error(err)
				return nil, err
			}
			ret[i].ExternalMetadata = &runtime.RawExtension{Raw: metadata}
		}

		if schemas := plans[i].Schemas; schemas != nil {
			if instanceSchemas := schemas.ServiceInstances; instanceSchemas != nil {
				if instanceCreateSchema := instanceSchemas.Create; instanceCreateSchema != nil && instanceCreateSchema.Parameters != nil {
					schema, err := json.Marshal(instanceCreateSchema.Parameters)
					if err != nil {
						err = fmt.Errorf("Failed to marshal instance create schema \n%+v\n %v", instanceCreateSchema.Parameters, err)
						glog.Error(err)
						return nil, err
					}
					ret[i].AlphaInstanceCreateParameterSchema = &runtime.RawExtension{Raw: schema}
				}
				if instanceUpdateSchema := instanceSchemas.Update; instanceUpdateSchema != nil && instanceUpdateSchema.Parameters != nil {
					schema, err := json.Marshal(instanceUpdateSchema.Parameters)
					if err != nil {
						err = fmt.Errorf("Failed to marshal instance update schema \n%+v\n %v", instanceUpdateSchema.Parameters, err)
						glog.Error(err)
						return nil, err
					}
					ret[i].AlphaInstanceUpdateParameterSchema = &runtime.RawExtension{Raw: schema}
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
					ret[i].AlphaBindingCreateParameterSchema = &runtime.RawExtension{Raw: schema}
				}
			}
		}

	}
	return ret, nil
}

func unmarshalParameters(in []byte) (map[string]interface{}, error) {
	parameters := make(map[string]interface{})
	if len(in) > 0 {
		if err := yaml.Unmarshal(in, &parameters); err != nil {
			return parameters, err
		}
	}
	return parameters, nil
}

// isInstanceReady returns whether the given instance has a ready condition
// with status true.
func isInstanceReady(instance *v1alpha1.Instance) bool {
	for _, cond := range instance.Status.Conditions {
		if cond.Type == v1alpha1.InstanceConditionReady {
			return cond.Status == v1alpha1.ConditionTrue
		}
	}

	return false
}
