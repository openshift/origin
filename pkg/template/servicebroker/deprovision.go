package servicebroker

import (
	"net/http"

	"github.com/openshift/origin/pkg/openservicebroker/api"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
)

func (b *Broker) Deprovision(instanceID string) *api.Response {
	brokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Get(instanceID)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return api.NewResponse(http.StatusGone, &api.DeprovisionResponse{}, nil)
		}
		return api.InternalServerError(err)
	}

	err = b.templateclient.TemplateInstances(brokerTemplateInstance.Spec.TemplateInstance.Namespace).Delete(brokerTemplateInstance.Spec.TemplateInstance.Name, kapi.NewPreconditionDeleteOptions(string(brokerTemplateInstance.Spec.TemplateInstance.UID)))
	if err != nil && !kerrors.IsNotFound(err) {
		return api.InternalServerError(err)
	}

	err = b.secretsGetter.Secrets(brokerTemplateInstance.Spec.Secret.Namespace).Delete(brokerTemplateInstance.Spec.Secret.Name, kapi.NewPreconditionDeleteOptions(string(brokerTemplateInstance.Spec.Secret.UID)))
	if err != nil && !kerrors.IsNotFound(err) {
		return api.InternalServerError(err)
	}

	opts := kapi.NewPreconditionDeleteOptions(string(brokerTemplateInstance.UID))
	err = b.templateclient.BrokerTemplateInstances().Delete(instanceID, opts)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return api.NewResponse(http.StatusGone, &api.DeprovisionResponse{}, nil)
		}
		return api.InternalServerError(err)
	}

	return api.NewResponse(http.StatusOK, &api.DeprovisionResponse{}, nil)
}
