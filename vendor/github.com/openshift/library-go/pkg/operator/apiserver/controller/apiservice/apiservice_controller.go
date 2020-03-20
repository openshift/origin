package apiservice

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1client "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"
	apiregistrationinformers "k8s.io/kube-aggregator/pkg/client/informers/externalversions"

	operatorsv1 "github.com/openshift/api/operator/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

type GetAPIServicesToMangeFunc func() ([]*apiregistrationv1.APIService, error)
type apiServicesPreconditionFuncType func([]*apiregistrationv1.APIService) (bool, error)

type APIServiceController struct {
	getAPIServicesToManageFn GetAPIServicesToMangeFunc
	// precondition must return true before the apiservices will be created
	precondition apiServicesPreconditionFuncType

	operatorClient          v1helpers.OperatorClient
	kubeClient              kubernetes.Interface
	apiregistrationv1Client apiregistrationv1client.ApiregistrationV1Interface
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
) factory.Controller {
	c := &APIServiceController{
		precondition:             newEndpointPrecondition(kubeInformersForOperandNamespace),
		getAPIServicesToManageFn: getAPIServicesToManageFunc,

		operatorClient:          operatorClient,
		apiregistrationv1Client: apiregistrationv1Client,
		kubeClient:              kubeClient,
	}

	return factory.New().WithSync(c.sync).ResyncEvery(10*time.Second).WithInformers(
		kubeInformersForOperandNamespace.Core().V1().Services().Informer(),
		kubeInformersForOperandNamespace.Core().V1().Endpoints().Informer(),
		apiregistrationInformers.Apiregistration().V1().APIServices().Informer(),
	).ToController("APIServiceController_"+name, eventRecorder.WithComponentSuffix("apiservice-"+name+"-controller"))
}

func (c *APIServiceController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
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
			if err := c.apiregistrationv1Client.APIServices().Delete(ctx, apiService.Name, metav1.DeleteOptions{}); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.NewAggregate(errs)
	default:
		syncCtx.Recorder().Warningf("ManagementStateUnknown", "Unrecognized operator management state %q", operatorConfigSpec.ManagementState)
		return nil
	}

	apiServices, err := c.getAPIServicesToManageFn()
	if err != nil {
		return err
	}
	ready, err := c.precondition(apiServices)
	if err != nil {
		if _, _, updateErr := v1helpers.UpdateStatus(c.operatorClient, v1helpers.UpdateConditionFn(operatorv1.OperatorCondition{
			Type:    "APIServicesAvailable",
			Status:  operatorv1.ConditionFalse,
			Reason:  "ErrorCheckingPrecondition",
			Message: err.Error(),
		})); updateErr != nil {
			return errors.NewAggregate([]error{err, updateErr})
		}
		return err
	}
	if !ready {
		if _, _, updateErr := v1helpers.UpdateStatus(c.operatorClient, v1helpers.UpdateConditionFn(operatorv1.OperatorCondition{
			Type:    "APIServicesAvailable",
			Status:  operatorv1.ConditionFalse,
			Reason:  "PreconditionNotReady",
			Message: "PreconditionNotReady",
		})); updateErr != nil {
			return errors.NewAggregate([]error{err, updateErr})
		}
		return err
	}

	err = c.syncAPIServices(apiServices, syncCtx.Recorder())

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

func (c *APIServiceController) syncAPIServices(apiServices []*apiregistrationv1.APIService, recorder events.Recorder) error {
	errs := []error{}
	var availableConditionMessages []string

	for _, apiService := range apiServices {
		apiregistrationv1.SetDefaults_ServiceReference(apiService.Spec.Service)
		apiService, _, err := resourceapply.ApplyAPIService(c.apiregistrationv1Client, recorder, apiService)
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
		missingAPIMessages := checkDiscoveryForByAPIServices(recorder, c.kubeClient.Discovery().RESTClient(), apiServices)
		availableConditionMessages = append(availableConditionMessages, missingAPIMessages...)
	}

	if len(availableConditionMessages) > 0 {
		sort.Sort(sort.StringSlice(availableConditionMessages))
		return fmt.Errorf(strings.Join(availableConditionMessages, "\n"))
	}

	return nil
}
