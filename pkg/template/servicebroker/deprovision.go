package servicebroker

import (
	"net/http"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/openservicebroker/api"
)

func (b *Broker) Deprovision(instanceID string) *api.Response {
	brokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return api.NewResponse(http.StatusGone, &api.DeprovisionResponse{}, nil)
		}
		return api.InternalServerError(err)
	}

	err = b.templateclient.TemplateInstances(brokerTemplateInstance.Spec.TemplateInstance.Namespace).Delete(brokerTemplateInstance.Spec.TemplateInstance.Name, metav1.NewPreconditionDeleteOptions(string(brokerTemplateInstance.Spec.TemplateInstance.UID)))
	if err != nil && !kerrors.IsNotFound(err) {
		return api.InternalServerError(err)
	}

	err = b.kc.Core().Secrets(brokerTemplateInstance.Spec.Secret.Namespace).Delete(brokerTemplateInstance.Spec.Secret.Name, metav1.NewPreconditionDeleteOptions(string(brokerTemplateInstance.Spec.Secret.UID)))
	if err != nil && !kerrors.IsNotFound(err) {
		return api.InternalServerError(err)
	}

	opts := metav1.NewPreconditionDeleteOptions(string(brokerTemplateInstance.UID))
	err = b.templateclient.BrokerTemplateInstances().Delete(instanceID, opts)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return api.NewResponse(http.StatusGone, &api.DeprovisionResponse{}, nil)
		}
		return api.InternalServerError(err)
	}

	return api.NewResponse(http.StatusOK, &api.DeprovisionResponse{}, nil)
}
