package apiservice

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1client "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"
	apiregistrationinformers "k8s.io/kube-aggregator/pkg/client/informers/externalversions"

	operatorsv1 "github.com/openshift/api/operator/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	operatorlistersv1 "github.com/openshift/client-go/operator/listers/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	workQueueKey = "key"
)

type GetAPIServicesToMangeFunc func() ([]*apiregistrationv1.APIService, error)
type apiServicesPreconditionFuncType func([]*apiregistrationv1.APIService) (bool, error)

type APIServiceController struct {
	name                     string
	getAPIServicesToManageFn GetAPIServicesToMangeFunc
	// precondition must return true before the apiservices will be created
	precondition apiServicesPreconditionFuncType

	operatorClient          v1helpers.OperatorClient
	kubeClient              kubernetes.Interface
	apiregistrationv1Client apiregistrationv1client.ApiregistrationV1Interface
	eventRecorder           events.Recorder

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewAPIServiceController(
	name string,
	getAPIServicesToManageFunc GetAPIServicesToMangeFunc,
	operatorClient v1helpers.OperatorClient,
	apiregistrationInformers apiregistrationinformers.SharedInformerFactory,
	apiregistrationv1Client apiregistrationv1client.ApiregistrationV1Interface,
	kubeInformersForOperandNamespace kubeinformers.SharedInformerFactory,
	kubeClient kubernetes.Interface,
	eventRecorder events.Recorder,
) *APIServiceController {
	fullname := "APIServiceController_" + name
	c := &APIServiceController{
		name:                     fullname,
		precondition:             newEndpointPrecondition(kubeInformersForOperandNamespace),
		getAPIServicesToManageFn: getAPIServicesToManageFunc,

		operatorClient:          operatorClient,
		apiregistrationv1Client: apiregistrationv1Client,
		kubeClient:              kubeClient,
		eventRecorder:           eventRecorder.WithComponentSuffix("apiservice-" + name + "-controller"),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fullname),
	}

	kubeInformersForOperandNamespace.Core().V1().Services().Informer().AddEventHandler(c.eventHandler())
	kubeInformersForOperandNamespace.Core().V1().Endpoints().Informer().AddEventHandler(c.eventHandler())
	apiregistrationInformers.Apiregistration().V1().APIServices().Informer().AddEventHandler(c.eventHandler())

	return c
}

func (c *APIServiceController) sync() error {
	operatorConfigSpec, _, _, err := c.operatorClient.GetOperatorState()
	if err != nil {
		return err
	}

	switch operatorConfigSpec.ManagementState {
	case operatorsv1.Managed:
	case operatorsv1.Unmanaged:
		return nil
	case operatorsv1.Removed:
		errs := []error{}
		apiServices, err := c.getAPIServicesToManageFn()
		if err != nil {
			errs = append(errs, err)
			return errors.NewAggregate(errs)
		}
		for _, apiService := range apiServices {
			if err := c.apiregistrationv1Client.APIServices().Delete(apiService.Name, nil); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.NewAggregate(errs)
	default:
		c.eventRecorder.Warningf("ManagementStateUnknown", "Unrecognized operator management state %q", operatorConfigSpec.ManagementState)
		return nil
	}

	apiServices, err := c.getAPIServicesToManageFn()
	if err != nil {
		return err
	}
	ready, err := c.precondition(apiServices)
	if err != nil {
		v1helpers.UpdateStatus(c.operatorClient, v1helpers.UpdateConditionFn(operatorv1.OperatorCondition{
			Type:    "APIServicesAvailable",
			Status:  operatorv1.ConditionFalse,
			Reason:  "ErrorCheckingPrecondition",
			Message: err.Error(),
		}))
		return err
	}
	if !ready {
		v1helpers.UpdateStatus(c.operatorClient, v1helpers.UpdateConditionFn(operatorv1.OperatorCondition{
			Type:    "APIServicesAvailable",
			Status:  operatorv1.ConditionFalse,
			Reason:  "PreconditionNotReady",
			Message: "PreconditionNotReady",
		}))
		return err
	}

	err = c.syncAPIServices(apiServices)

	// update failing condition
	cond := operatorv1.OperatorCondition{
		Type:   "APIServicesAvailable",
		Status: operatorv1.ConditionTrue,
	}
	if err != nil {
		cond.Status = operatorv1.ConditionFalse
		cond.Reason = "Error"
		cond.Message = err.Error()
	}
	if _, _, updateError := v1helpers.UpdateStatus(c.operatorClient, v1helpers.UpdateConditionFn(cond)); updateError != nil {
		if err == nil {
			return updateError
		}
	}

	return err
}

func (c *APIServiceController) syncAPIServices(apiServices []*apiregistrationv1.APIService) error {
	errs := []error{}
	var availableConditionMessages []string

	for _, apiService := range apiServices {
		apiregistrationv1.SetDefaults_ServiceReference(apiService.Spec.Service)
		apiService, _, err := resourceapply.ApplyAPIService(c.apiregistrationv1Client, c.eventRecorder, apiService)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, condition := range apiService.Status.Conditions {
			if condition.Type == apiregistrationv1.Available {
				if condition.Status != apiregistrationv1.ConditionTrue {
					availableConditionMessages = append(availableConditionMessages, fmt.Sprintf("apiservices.apiregistration.k8s.io/%v: not available: %v", apiService.Name, condition.Message))
				}
				break
			}
		}
	}
	if len(errs) > 0 {
		return errors.NewAggregate(errs)
	}
	if len(availableConditionMessages) > 0 {
		sort.Sort(sort.StringSlice(availableConditionMessages))
		return fmt.Errorf(strings.Join(availableConditionMessages, "\n"))
	}

	// if the apiservices themselves check out ok, try to actually hit the discovery endpoints.  We have a history in clusterup
	// of something delaying them.  This isn't perfect because of round-robining, but let's see if we get an improvement
	if c.kubeClient.Discovery().RESTClient() != nil {
		missingAPIMessages := checkDiscoveryForByAPIServices(c.eventRecorder, c.kubeClient.Discovery().RESTClient(), apiServices)
		availableConditionMessages = append(availableConditionMessages, missingAPIMessages...)
	}

	if len(availableConditionMessages) > 0 {
		sort.Sort(sort.StringSlice(availableConditionMessages))
		return fmt.Errorf(strings.Join(availableConditionMessages, "\n"))
	}

	return nil
}

// Run starts the openshift-apiserver and blocks until stopCh is closed.
// The number of workers is ignored
func (c *APIServiceController) Run(ctx context.Context, _ int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting %v", c.name)
	defer klog.Infof("Shutting down %v", c.name)

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, ctx.Done())

	<-ctx.Done()
}

func (c *APIServiceController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *APIServiceController) processNextWorkItem() bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	err := c.sync()
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}

// eventHandler queues the operator to check spec and status
func (c *APIServiceController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(workQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(workQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(workQueueKey) },
	}
}

// APIServicesToMange preserve state and clients required to return an authoritative list of API services this operate must manage
type APIServicesToManage struct {
	authOperatorLister                         operatorlistersv1.AuthenticationLister
	apiregistrationv1Client                    apiregistrationv1client.ApiregistrationV1Interface
	allPossibleAPIServices                     []*apiregistrationv1.APIService
	eventRecorder                              events.Recorder
	apiGroupsManagedByExternalServer           sets.String
	apiGroupsManagedByExternalServerAnnotation string
	currentAPIServicesToManage                 []*apiregistrationv1.APIService
}

// NewAPIServicesToManage returns an object that knows how to construct an authoritative list of API services this operate must manage
func NewAPIServicesToManage(apiregistrationv1Client apiregistrationv1client.ApiregistrationV1Interface,
	authOperatorLister operatorlistersv1.AuthenticationLister,
	allPossibleAPIServices []*apiregistrationv1.APIService,
	eventRecorder events.Recorder,
	apiGroupsManagedByExternalServer sets.String,
	apiGroupsManagedByExternalServerAnnotation string) *APIServicesToManage {
	return &APIServicesToManage{
		authOperatorLister:                         authOperatorLister,
		apiregistrationv1Client:                    apiregistrationv1Client,
		allPossibleAPIServices:                     allPossibleAPIServices,
		eventRecorder:                              eventRecorder,
		apiGroupsManagedByExternalServer:           apiGroupsManagedByExternalServer,
		apiGroupsManagedByExternalServerAnnotation: apiGroupsManagedByExternalServerAnnotation,
		currentAPIServicesToManage:                 allPossibleAPIServices,
	}
}

// GetAPIServicesToManage returns the desired list of API Services that will be managed by this operator
// note that some services might be managed by an external operators/servers
func (a *APIServicesToManage) GetAPIServicesToManage() ([]*apiregistrationv1.APIService, error) {
	if externalOperatorPreconditionErr := a.externalOperatorPrecondition(); externalOperatorPreconditionErr != nil {
		klog.V(4).Infof("unable to determine if an external operator should take OAuth APIs over due to %v, returning authoritative/initial API Services list", externalOperatorPreconditionErr)
		return a.allPossibleAPIServices, nil
	}

	newAPIServicesToManage := []*apiregistrationv1.APIService{}
	for _, apiService := range a.allPossibleAPIServices {
		if a.apiGroupsManagedByExternalServer.Has(apiService.Name) && a.isAPIServiceAnnotatedByExternalServer(apiService) {
			continue
		}
		newAPIServicesToManage = append(newAPIServicesToManage, apiService)
	}

	if changed, newAPIServicesSet := apiServicesChanged(a.currentAPIServicesToManage, newAPIServicesToManage); changed {
		a.eventRecorder.Eventf("APIServicesToManageChanged", "The new API Services list this operator will manage is %v", newAPIServicesSet.List())
	}

	a.currentAPIServicesToManage = newAPIServicesToManage
	return a.currentAPIServicesToManage, nil
}

func (a *APIServicesToManage) isAPIServiceAnnotatedByExternalServer(apiService *apiregistrationv1.APIService) bool {
	existingApiService, err := a.apiregistrationv1Client.APIServices().Get(apiService.Name, metav1.GetOptions{})
	if err != nil {
		a.eventRecorder.Warningf("APIServicesToManageAnnotation", "unable to determine if the following API Service %s was annotated by an external operator (it should be) due to %v", apiService.Name, err)
		return false
	}

	if _, ok := existingApiService.Annotations[a.apiGroupsManagedByExternalServerAnnotation]; ok {
		return true

	}
	return false
}

// externalOperatorPrecondition checks whether authentication operator will manage OAuth API Resources by checking ManagingOAuthAPIServer status field
func (a *APIServicesToManage) externalOperatorPrecondition() error {
	authOperator, err := a.authOperatorLister.Get("cluster")
	if err != nil {
		return err
	}

	if !authOperator.Status.ManagingOAuthAPIServer {
		return fmt.Errorf("%q status field set to false", "ManagingOAuthAPIServer")
	}

	return nil
}

func apiServicesChanged(old []*apiregistrationv1.APIService, new []*apiregistrationv1.APIService) (bool, sets.String) {
	oldSet := sets.String{}
	for _, oldService := range old {
		oldSet.Insert(oldService.Name)
	}

	newSet := sets.String{}
	for _, newService := range new {
		newSet.Insert(newService.Name)
	}

	removed := oldSet.Difference(newSet).List()
	added := newSet.Difference(oldSet).List()
	return len(removed) > 0 || len(added) > 0, newSet
}
