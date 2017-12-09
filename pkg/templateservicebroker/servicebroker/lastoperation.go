package servicebroker

import (
	"errors"
	"net/http"

	"github.com/golang/glog"

	authorizationv1 "k8s.io/api/authorization/v1"
	kapiv1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"

	templateapiv1 "github.com/openshift/api/template/v1"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
	"github.com/openshift/origin/pkg/templateservicebroker/util"
)

// LastOperation returns the status of an asynchronous operation.  Currently
// the OSB API only supports async Provision and Deprovision.
func (b *Broker) LastOperation(u user.Info, instanceID string, operation api.Operation) *api.Response {
	glog.V(4).Infof("Template service broker: LastOperation: instanceID %s", instanceID)

	switch operation {
	case api.OperationProvisioning:
		return b.lastOperationProvisioning(u, instanceID)
	case api.OperationDeprovisioning:
		return b.lastOperationDeprovisioning(u, instanceID)
	}

	return api.BadRequest(errors.New("invalid operation"))
}

// lastOperationProvisioning returns the status of an asynchronous provision
// operation.
func (b *Broker) lastOperationProvisioning(u user.Info, instanceID string) *api.Response {
	brokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return api.BadRequest(err)
		}
		return api.InternalServerError(err)
	}

	namespace := brokerTemplateInstance.Spec.TemplateInstance.Namespace

	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
		Namespace: namespace,
		Verb:      "get",
		Group:     templateapi.GroupName,
		Resource:  "templateinstances",
		Name:      brokerTemplateInstance.Spec.TemplateInstance.Name,
	}); err != nil {
		return api.Forbidden(err)
	}

	templateInstance, err := b.templateclient.TemplateInstances(namespace).Get(brokerTemplateInstance.Spec.TemplateInstance.Name, metav1.GetOptions{})
	if err != nil {
		return api.InternalServerError(err)
	}

	state := api.LastOperationStateInProgress
	var description string
	for _, condition := range templateInstance.Status.Conditions {
		if condition.Type == templateapiv1.TemplateInstanceReady && condition.Status == kapiv1.ConditionTrue {
			state = api.LastOperationStateSucceeded
			break
		}
		if condition.Type == templateapiv1.TemplateInstanceInstantiateFailure && condition.Status == kapiv1.ConditionTrue {
			state = api.LastOperationStateFailed
			description = condition.Message
			break
		}
	}

	return api.NewResponse(http.StatusOK, &api.LastOperationResponse{State: state, Description: description}, nil)
}

// lastOperationDerovisioning returns the status of an asynchronous deprovision
// operation.
func (b *Broker) lastOperationDeprovisioning(u user.Info, instanceID string) *api.Response {
	brokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return api.NewResponse(http.StatusOK, &api.LastOperationResponse{State: api.LastOperationStateSucceeded}, nil)
		}
		return api.InternalServerError(err)
	}

	namespace := brokerTemplateInstance.Spec.TemplateInstance.Namespace

	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
		Namespace: namespace,
		Verb:      "get",
		Group:     templateapi.GroupName,
		Resource:  "templateinstances",
		Name:      brokerTemplateInstance.Spec.TemplateInstance.Name,
	}); err != nil {
		return api.Forbidden(err)
	}

	return api.NewResponse(http.StatusOK, &api.LastOperationResponse{State: api.LastOperationStateInProgress}, nil)
}
