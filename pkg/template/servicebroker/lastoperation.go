package servicebroker

import (
	"errors"
	"net/http"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/openservicebroker/api"
	templateapi "github.com/openshift/origin/pkg/template/api"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"
)

// LastOperation returns the status of an asynchronous operation.  Currently
// the OSB API only supports async Provision and Deprovision; we don't currently
// support async Deprovision as the garbage collector doesn't indicate when it's
// done cleaning up after a given object is removed.
func (b *Broker) LastOperation(instanceID string, operation api.Operation) *api.Response {
	// TODO: currently the spec does not allow for user information to be
	// provided on LastOperation, so little authorization can be carried out.

	glog.V(4).Infof("Template service broker: LastOperation: instanceID %s", instanceID)

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
