package servicebroker

import (
	"net/http"

	"github.com/golang/glog"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/authorization"

	"github.com/openshift/origin/pkg/authorization/util"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
)

// Deprovision is the reverse of Provision.  We clean up the TemplateInstance,
// Secret and BrokerTemplateInstance objects (in that order); the garbage
// collector is responsible for the removal of the objects provisioned by the
// Template(Instance) itself.
func (b *Broker) Deprovision(u user.Info, instanceID string) *api.Response {
	glog.V(4).Infof("Template service broker: Deprovision: instanceID %s", instanceID)

	brokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return api.NewResponse(http.StatusGone, &api.DeprovisionResponse{}, nil)
		}
		return api.InternalServerError(err)
	}

	namespace := brokerTemplateInstance.Spec.TemplateInstance.Namespace

	// end users are not expected to have access to BrokerTemplateInstance
	// objects; SAR on the TemplateInstance instead.

	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
		Namespace: namespace,
		Verb:      "get",
		Group:     templateapi.GroupName,
		Resource:  "templateinstances",
		Name:      brokerTemplateInstance.Spec.TemplateInstance.Name,
	}); err != nil {
		return api.Forbidden(err)
	}

	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
		Namespace: namespace,
		Verb:      "delete",
		Group:     templateapi.GroupName,
		Resource:  "templateinstances",
		Name:      brokerTemplateInstance.Spec.TemplateInstance.Name,
	}); err != nil {
		return api.Forbidden(err)
	}

	err = b.templateclient.TemplateInstances(namespace).Delete(brokerTemplateInstance.Spec.TemplateInstance.Name, metav1.NewPreconditionDeleteOptions(string(brokerTemplateInstance.Spec.TemplateInstance.UID)))
	if err != nil && !kerrors.IsNotFound(err) {
		return api.InternalServerError(err)
	}

	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
		Namespace: namespace,
		Verb:      "delete",
		Group:     kapi.GroupName,
		Resource:  "secrets",
		Name:      brokerTemplateInstance.Spec.Secret.Name,
	}); err != nil {
		return api.Forbidden(err)
	}

	err = b.kc.Core().Secrets(brokerTemplateInstance.Spec.Secret.Namespace).Delete(brokerTemplateInstance.Spec.Secret.Name, metav1.NewPreconditionDeleteOptions(string(brokerTemplateInstance.Spec.Secret.UID)))
	if err != nil && !kerrors.IsNotFound(err) {
		return api.InternalServerError(err)
	}

	// The OSB API requires this function to be idempotent (restartable).  If
	// any actual change was made, per the spec, StatusOK is returned, else
	// StatusGone.

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
