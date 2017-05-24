package servicebroker

import (
	"net/http"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/openservicebroker/api"
)

// Unbind is the reverse of Bind.  Currently it simply removes the binding ID
// from the BrokerTemplateInstance, if found.
func (b *Broker) Unbind(instanceID, bindingID string) *api.Response {
	glog.V(4).Infof("Template service broker: Unbind: instanceID %s, bindingID %s", instanceID, bindingID)

	// TODO: currently the spec does not allow for user information to be
	// provided on Unbind, so little authorization can be carried out.

	brokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return api.BadRequest(err)
		}

		return api.InternalServerError(err)
	}

	// The OSB API requires this function to be idempotent (restartable).  If
	// any actual change was made, per the spec, StatusOK is returned, else
	// StatusGone.

	status := http.StatusGone
	for i := 0; i < len(brokerTemplateInstance.Spec.BindingIDs); i++ {
		for i < len(brokerTemplateInstance.Spec.BindingIDs) && brokerTemplateInstance.Spec.BindingIDs[i] == bindingID {
			brokerTemplateInstance.Spec.BindingIDs = append(brokerTemplateInstance.Spec.BindingIDs[:i], brokerTemplateInstance.Spec.BindingIDs[i+1:]...)
			status = http.StatusOK
		}
	}
	if status == http.StatusOK { // binding found; remove it
		brokerTemplateInstance, err = b.templateclient.BrokerTemplateInstances().Update(brokerTemplateInstance)
		if err != nil {
			return api.InternalServerError(err)
		}
	}

	return api.NewResponse(status, &api.UnbindResponse{}, nil)
}
