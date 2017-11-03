package servicebroker

import (
	"net/http"
	"time"

	"github.com/golang/glog"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
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

	templateInstance := brokerTemplateInstance.Spec.TemplateInstance
	opts := metav1.NewPreconditionDeleteOptions(string(templateInstance.UID))
	policy := metav1.DeletePropagationForeground
	opts.PropagationPolicy = &policy
	err = b.templateclient.TemplateInstances(templateInstance.Namespace).Delete(templateInstance.Name, opts)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return api.InternalServerError(err)
		}
	}
	// Wait for the template instance to go away due to foreground GC propagation
	err = wait.Poll(time.Second, 30*time.Second, func() (bool, error) {
		v, err := b.templateclient.TemplateInstances(templateInstance.Namespace).Get(templateInstance.Name, metav1.GetOptions{})
		if err == nil && v == nil {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})
	// If we timed out or got another error while waiting for it to go away, abort the deprovision.
	// (since the broker template instance will still be around, the de-provision can be retried)
	if err != nil {
		return api.InternalServerError(err)
	}
	opts = metav1.NewPreconditionDeleteOptions(string(brokerTemplateInstance.UID))
	opts.PropagationPolicy = &policy

	err = b.templateclient.BrokerTemplateInstances().Delete(instanceID, opts)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return api.NewResponse(http.StatusGone, &api.DeprovisionResponse{}, nil)
		}
		return api.InternalServerError(err)
	}

	return api.NewResponse(http.StatusAccepted, &api.DeprovisionResponse{Operation: api.OperationDeprovisioning}, nil)
}
