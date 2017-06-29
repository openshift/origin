package servicebroker

import (
	"net/http"
	"reflect"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/authorization"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/authorization/util"
	"github.com/openshift/origin/pkg/openservicebroker/api"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

// ensureSecret ensures the existence of a Secret object containing the template
// configuration parameters.
func (b *Broker) ensureSecret(u user.Info, namespace string, instanceID string, preq *api.ProvisionRequest, didWork *bool) (*kapi.Secret, *api.Response) {
	glog.V(4).Infof("Template service broker: ensureSecret")

	secret := &kapi.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: instanceID},
		Data:       map[string][]byte{},
	}

	for k, v := range preq.Parameters {
		if k != templateapi.RequesterUsernameParameterKey {
			secret.Data[k] = []byte(v)
		}
	}

	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
		Namespace: namespace,
		Verb:      "create",
		Group:     kapi.GroupName,
		Resource:  "secrets",
		Name:      secret.Name,
	}); err != nil {
		return nil, api.Forbidden(err)
	}

	createdSec, err := b.kc.Core().Secrets(namespace).Create(secret)
	if err == nil {
		*didWork = true
		return createdSec, nil
	}

	if kerrors.IsAlreadyExists(err) {
		if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
			Namespace: namespace,
			Verb:      "get",
			Group:     kapi.GroupName,
			Resource:  "secrets",
			Name:      secret.Name,
		}); err != nil {
			return nil, api.Forbidden(err)
		}

		existingSec, err := b.kc.Core().Secrets(namespace).Get(secret.Name, metav1.GetOptions{})
		if err == nil && reflect.DeepEqual(secret.Data, existingSec.Data) {
			return existingSec, nil
		}

		return nil, api.NewResponse(http.StatusConflict, api.ProvisionResponse{}, nil)
	}

	if kerrors.IsForbidden(err) {
		return nil, api.Forbidden(err)
	}
	return nil, api.InternalServerError(err)
}

// ensureTemplateInstance ensures the existence of a TemplateInstance object
// (this causes the template instance controller to instantiate the template in
// the namespace).
func (b *Broker) ensureTemplateInstance(u user.Info, namespace string, instanceID string, template *templateapi.Template, secret *kapi.Secret, didWork *bool) (*templateapi.TemplateInstance, *api.Response) {
	glog.V(4).Infof("Template service broker: ensureTemplateInstance")

	templateInstance := &templateapi.TemplateInstance{
		ObjectMeta: metav1.ObjectMeta{Name: instanceID},
		Spec: templateapi.TemplateInstanceSpec{
			Template: *template,
			Secret:   &kapi.LocalObjectReference{Name: secret.Name},
			Requester: &templateapi.TemplateInstanceRequester{
				Username: u.GetName(),
			},
		},
	}

	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
		Namespace: namespace,
		Verb:      "create",
		Group:     templateapi.GroupName,
		Resource:  "templateinstances",
	}); err != nil {
		return nil, api.Forbidden(err)
	}

	createdTemplateInstance, err := b.templateclient.TemplateInstances(namespace).Create(templateInstance)
	if err == nil {
		*didWork = true
		return createdTemplateInstance, nil
	}

	if kerrors.IsAlreadyExists(err) {
		if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
			Namespace: namespace,
			Verb:      "get",
			Group:     templateapi.GroupName,
			Resource:  "templateinstances",
			Name:      templateInstance.Name,
		}); err != nil {
			return nil, api.Forbidden(err)
		}

		existingTemplateInstance, err := b.templateclient.TemplateInstances(namespace).Get(templateInstance.Name, metav1.GetOptions{})
		if err == nil && reflect.DeepEqual(templateInstance.Spec, existingTemplateInstance.Spec) {
			return existingTemplateInstance, nil
		}

		return nil, api.NewResponse(http.StatusConflict, api.ProvisionResponse{}, nil)
	}

	if kerrors.IsForbidden(err) {
		return nil, api.Forbidden(err)
	}
	return nil, api.InternalServerError(err)
}

// ensureBrokerTemplateInstanceUIDs ensures the UIDs of the namespaced Secret
// and TemplateInstance objects are set in the BrokerTemplateInstance object, as
// proof that we are done.
func (b *Broker) ensureBrokerTemplateInstanceUIDs(brokerTemplateInstance *templateapi.BrokerTemplateInstance, secret *kapi.Secret, templateInstance *templateapi.TemplateInstance, didWork *bool) (*templateapi.BrokerTemplateInstance, *api.Response) {
	glog.V(4).Infof("Template service broker: ensureBrokerTemplateInstanceUIDs")

	brokerTemplateInstance.Spec.Secret.UID = secret.UID
	brokerTemplateInstance.Spec.TemplateInstance.UID = templateInstance.UID

	// there is no SAR here because end users are not expected to have access to
	// BrokerTemplateInstance objects.

	brokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Update(brokerTemplateInstance)
	if err == nil {
		*didWork = true
		return brokerTemplateInstance, nil
	}

	return nil, api.InternalServerError(err)
}

// ensureBrokerTemplateInstance ensures the existence of BrokerTemplateInstance
// object (records intent, globally maps instanceID to namespaced Secret and
// TemplateInstance objects).
func (b *Broker) ensureBrokerTemplateInstance(namespace, instanceID string, didWork *bool) (*templateapi.BrokerTemplateInstance, *api.Response) {
	glog.V(4).Infof("Template service broker: ensureBrokerTemplateInstance")

	brokerTemplateInstance := &templateapi.BrokerTemplateInstance{
		ObjectMeta: metav1.ObjectMeta{Name: instanceID},
		Spec: templateapi.BrokerTemplateInstanceSpec{
			TemplateInstance: kapi.ObjectReference{
				Kind:      "TemplateInstance",
				Namespace: namespace,
				Name:      instanceID,
			},
			Secret: kapi.ObjectReference{
				Kind:      "Secret",
				Namespace: namespace,
				Name:      instanceID,
			},
		},
	}

	// there is no SAR here because end users are not expected to have access to
	// BrokerTemplateInstance objects.

	newBrokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Create(brokerTemplateInstance)
	if err == nil {
		*didWork = true
		return newBrokerTemplateInstance, nil
	}

	if kerrors.IsAlreadyExists(err) {
		existingBrokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Get(brokerTemplateInstance.Name, metav1.GetOptions{})
		if err == nil && reflect.DeepEqual(brokerTemplateInstance.Spec, existingBrokerTemplateInstance.Spec) {
			return existingBrokerTemplateInstance, nil
		}

		return nil, api.NewResponse(http.StatusConflict, api.ProvisionResponse{}, nil)
	}

	return nil, api.InternalServerError(err)
}

// Provision instantiates a template from a ProvisionRequest, via the OpenShift
// TemplateInstance API.
func (b *Broker) Provision(instanceID string, preq *api.ProvisionRequest) *api.Response {
	glog.V(4).Infof("Template service broker: Provision: instanceID %s", instanceID)

	if errs := ValidateProvisionRequest(preq); len(errs) > 0 {
		return api.BadRequest(errs.ToAggregate())
	}

	namespace := preq.Context.Namespace
	impersonate := preq.Parameters[templateapi.RequesterUsernameParameterKey]
	u := &user.DefaultInfo{Name: impersonate}

	template, err := b.lister.GetByUID(preq.ServiceID)
	if err != nil && !kerrors.IsNotFound(err) {
		return api.BadRequest(err)
	}
	if template == nil {
		// If the template is not found, it is just possible that it is because
		// the cache is out of date.  To be sure, fall back to O(N) search of
		// templates in configured namespace(s).
		glog.V(4).Infof("Template service broker: GetByUID didn't template %s", preq.ServiceID)

	out:
		for namespace := range b.templateNamespaces {
			templates, err := b.lister.Templates(namespace).List(labels.Everything())
			if err != nil {
				return api.InternalServerError(err)
			}
			for _, t := range templates {
				if string(t.UID) == preq.ServiceID {
					template = t
					break out
				}
			}
		}
	}
	if template == nil {
		glog.V(4).Infof("Template service broker: template %s not found", preq.ServiceID)
		return api.BadRequest(kerrors.NewNotFound(templateapi.Resource("templates"), preq.ServiceID))
	}
	if _, ok := b.templateNamespaces[template.Namespace]; !ok {
		return api.BadRequest(kerrors.NewNotFound(templateapi.Resource("templates"), preq.ServiceID))
	}

	// TODO: enable SAR for template - at the moment I think this doesn't work
	// properly because group information isn't populated in u.
	/*
		if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
			Namespace: template.Namespace,
			Verb:      "get",
			Group:     templateapi.GroupName,
			Resource:  "templates",
			Name:      template.Name,
		}); err != nil {
			return api.Forbidden(err)
		}
	*/

	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
		Namespace: namespace,
		Verb:      "create",
		Group:     templateapi.GroupName,
		Resource:  "templateinstances",
		Name:      instanceID,
	}); err != nil {
		return api.Forbidden(err)
	}

	// The OSB API requires this function to be idempotent (restartable).  Thus
	// each sub-step is intended to succeed if it sets the desired state, or if
	// the desired state is already set.  didWork tracks if any actual change
	// was made (if so, per the spec, StatusAccepted is returned, else
	// StatusOK).

	didWork := false

	// The flow is as follows:
	// 1. Ensure existence of BrokerTemplateInstance (records intent, globally
	// maps instanceID to namespaced Secret and TemplateInstance objects).
	// 2. Ensure existence of Secret containing template configuration
	// parameters.
	// 3. Ensure existence of TemplateInstance object (this causes the template
	// instance controller to instantiate the template in the namespace).
	// 4. Ensure the UIDs of the namespaced Secret and TemplateInstance objects
	// are set in the BrokerTemplateInstance object, as proof that we are done.

	brokerTemplateInstance, resp := b.ensureBrokerTemplateInstance(namespace, instanceID, &didWork)
	if resp != nil {
		return resp
	}

	secret, resp := b.ensureSecret(u, namespace, instanceID, preq, &didWork)
	if resp != nil {
		return resp
	}

	templateInstance, resp := b.ensureTemplateInstance(u, namespace, instanceID, template, secret, &didWork)
	if resp != nil {
		return resp
	}

	_, resp = b.ensureBrokerTemplateInstanceUIDs(brokerTemplateInstance, secret, templateInstance, &didWork)
	if resp != nil {
		return resp
	}

	if didWork {
		return api.NewResponse(http.StatusAccepted, api.ProvisionResponse{Operation: api.OperationProvisioning}, nil)
	}

	return api.NewResponse(http.StatusOK, api.ProvisionResponse{Operation: api.OperationProvisioning}, nil)
}
