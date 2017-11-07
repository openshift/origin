package servicebroker

import (
	"errors"
	"net/http"
	"strings"

	"github.com/golang/glog"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/kubernetes/pkg/apis/authorization"

	"github.com/openshift/origin/pkg/authorization/util"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
)

// Unbind is the reverse of Bind.  Currently it simply removes the binding ID
// from the BrokerTemplateInstance, if found.
func (b *Broker) Unbind(u user.Info, instanceID, bindingID string) *api.Response {
	glog.V(4).Infof("Template service broker: Unbind: instanceID %s, bindingID %s", instanceID, bindingID)

	brokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return api.BadRequest(err)
		}

		return api.InternalServerError(err)
	}

	namespace := brokerTemplateInstance.Spec.TemplateInstance.Namespace

	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
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
	if strings.ToLower(templateInstance.Spec.Template.Annotations[templateapi.BindableAnnotation]) == "false" {
		return api.BadRequest(errors.New("provisioned service is not bindable"))
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
		// end users are not expected to have access to BrokerTemplateInstance
		// objects; SAR on the TemplateInstance instead.
		if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
			Namespace: namespace,
			Verb:      "update",
			Group:     templateapi.GroupName,
			Resource:  "templateinstances",
			Name:      brokerTemplateInstance.Spec.TemplateInstance.Name,
		}); err != nil {
			return api.Forbidden(err)
		}

		brokerTemplateInstance, err = b.templateclient.BrokerTemplateInstances().Update(brokerTemplateInstance)
		if err != nil {
			return api.InternalServerError(err)
		}
	}

	return api.NewResponse(status, &api.UnbindResponse{}, nil)
}
