package servicebroker

import (
	"errors"
	"net/http"
	"strings"

	"github.com/golang/glog"

	authorizationv1 "k8s.io/api/authorization/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/util/retry"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
	"github.com/openshift/origin/pkg/templateservicebroker/util"
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
		// Tolerate NotFound errors in case the user deleted the
		// templateinstance manually.  We do not want to leave the system in a
		// state where unbind cannot proceed because then the user cannot
		// deprovision either(you must unbind before deprovisioning).  So just
		// proceed as if we found it.
		if !kerrors.IsNotFound(err) {
			return api.InternalServerError(err)
		}
	}
	if templateInstance != nil && strings.ToLower(templateInstance.Spec.Template.Annotations[templateapi.BindableAnnotation]) == "false" {
		return api.BadRequest(errors.New("provisioned service is not bindable"))
	}

	// end users are not expected to have access to BrokerTemplateInstance
	// objects; SAR on the TemplateInstance instead.
	// Note that this specific templateinstance object might not actually exist
	// anymore, but the SAR check is still valid to confirm the user can update
	// templateinstances in this namespace.
	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
		Namespace: namespace,
		Verb:      "delete",
		Group:     templateapi.GroupName,
		Resource:  "templateinstances",
		Name:      brokerTemplateInstance.Spec.TemplateInstance.Name,
	}); err != nil {
		return api.Forbidden(err)
	}

	// The OSB API requires this function to be idempotent (restartable).  If
	// any actual change was made, per the spec, StatusOK is returned, else
	// StatusGone.

	var status int
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		status = http.StatusGone
		for i := 0; i < len(brokerTemplateInstance.Spec.BindingIDs); i++ {
			for i < len(brokerTemplateInstance.Spec.BindingIDs) && brokerTemplateInstance.Spec.BindingIDs[i] == bindingID {
				brokerTemplateInstance.Spec.BindingIDs = append(brokerTemplateInstance.Spec.BindingIDs[:i], brokerTemplateInstance.Spec.BindingIDs[i+1:]...)
				status = http.StatusOK
			}
		}
		if status == http.StatusGone {
			return nil
		}

		// binding found; remove it
		newBrokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Update(brokerTemplateInstance)
		switch {
		case err == nil:
			brokerTemplateInstance = newBrokerTemplateInstance

		case kerrors.IsConflict(err):
			var getErr error
			brokerTemplateInstance, getErr = b.templateclient.BrokerTemplateInstances().Get(brokerTemplateInstance.Name, metav1.GetOptions{})
			if getErr != nil {
				err = getErr
			}
		}
		return err
	})
	switch {
	case err == nil:
		return api.NewResponse(status, &api.UnbindResponse{}, nil)
	case kerrors.IsConflict(err):
		return api.NewResponse(http.StatusUnprocessableEntity, &api.ConcurrencyError, nil)
	}
	return api.InternalServerError(err)
}
