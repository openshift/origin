package servicebroker

import (
	"errors"
	"net/http"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/authorization/util"
	"github.com/openshift/origin/pkg/openservicebroker/api"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/authorization"
)

// LastOperation returns the status of an asynchronous operation.  Currently
// the OSB API only supports async Provision and Deprovision; we don't currently
// support async Deprovision as the garbage collector doesn't indicate when it's
// done cleaning up after a given object is removed.
func (b *Broker) LastOperation(u user.Info, instanceID string, operation api.Operation) *api.Response {
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

	namespace := brokerTemplateInstance.Spec.TemplateInstance.Namespace

	//TODO - when https://github.com/kubernetes-incubator/service-catalog/pull/939 sufficiently progresses, remove the user name empty string checks
	if u.GetName() != "" {
		if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
			Namespace: namespace,
			Verb:      "get",
			Group:     templateapi.GroupName,
			Resource:  "templateinstances",
			Name:      brokerTemplateInstance.Spec.TemplateInstance.Name,
		}); err != nil {
			return api.Forbidden(err)
		}
	}

	templateInstance, err := b.templateclient.TemplateInstances(namespace).Get(brokerTemplateInstance.Spec.TemplateInstance.Name, metav1.GetOptions{})
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
