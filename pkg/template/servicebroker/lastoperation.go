package servicebroker

import (
	"errors"
	"net/http"

	"github.com/openshift/origin/pkg/openservicebroker/api"
	templateapi "github.com/openshift/origin/pkg/template/api"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"
)

func (b *Broker) LastOperation(instanceID string, operation api.Operation) *api.Response {
	if operation != api.OperationProvisioning {
		return api.BadRequest(errors.New("invalid operation"))
	}

	brokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return api.BadRequest(err)
		}
		return api.InternalServerError(err)
	}

	templateInstance, err := b.templateclient.TemplateInstances(brokerTemplateInstance.Spec.TemplateInstance.Namespace).Get(brokerTemplateInstance.Spec.TemplateInstance.Name, metav1.GetOptions{})
	if err != nil {
		return api.InternalServerError(err)
	}

	state := api.LastOperationStateInProgress
	for _, condition := range templateInstance.Status.Conditions {
		if condition.Type == templateapi.TemplateInstanceReady && condition.Status == kapi.ConditionTrue {
			state = api.LastOperationStateSucceeded
			break
		}
		if condition.Type == templateapi.TemplateInstanceInstantiateFailure && condition.Status == kapi.ConditionTrue {
			state = api.LastOperationStateFailed
			break
		}
	}

	return api.NewResponse(http.StatusOK, &api.LastOperationResponse{State: state}, nil)
}
