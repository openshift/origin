package servicebroker

import (
	"net/http"

	"github.com/openshift/origin/pkg/openservicebroker/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
)

func (b *Broker) Unbind(instanceID, bindingID string) *api.Response {
	brokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Get(instanceID)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return api.BadRequest(err)
		}

		return api.InternalServerError(err)
	}

	status := http.StatusGone
	for i := 0; i < len(brokerTemplateInstance.Spec.BindingIDs); i++ {
		for i < len(brokerTemplateInstance.Spec.BindingIDs) && brokerTemplateInstance.Spec.BindingIDs[i] == bindingID {
			brokerTemplateInstance.Spec.BindingIDs = append(brokerTemplateInstance.Spec.BindingIDs[:i], brokerTemplateInstance.Spec.BindingIDs[i+1:]...)
			status = http.StatusOK
		}
	}
	if status == http.StatusOK {
		brokerTemplateInstance, err = b.templateclient.BrokerTemplateInstances().Update(brokerTemplateInstance)
		if err != nil {
			return api.InternalServerError(err)
		}
	}

	return api.NewResponse(status, &api.UnbindResponse{}, nil)
}
