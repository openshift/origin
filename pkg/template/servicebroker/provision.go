package servicebroker

import (
	"net/http"
	"reflect"

	"github.com/openshift/origin/pkg/openservicebroker/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	templateclientset "github.com/openshift/origin/pkg/template/clientset/internalclientset"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

func (b *Broker) ensureSecret(impersonatedKC *kclientset.Clientset, namespace string, instanceID string, preq *api.ProvisionRequest, didWork *bool) (*kapi.Secret, *api.Response) {
	secret := &kapi.Secret{
		ObjectMeta: kapi.ObjectMeta{Name: instanceID},
		Data:       map[string][]byte{},
	}

	for k, v := range preq.Parameters {
		if k != templateapi.NamespaceParameterKey && k != templateapi.RequesterUsernameParameterKey {
			secret.Data[k] = []byte(v)
		}
	}

	createdSec, err := impersonatedKC.Secrets(namespace).Create(secret)
	if err == nil {
		*didWork = true
		return createdSec, nil
	}

	if kerrors.IsAlreadyExists(err) {
		existingSec, err := impersonatedKC.Secrets(namespace).Get(secret.Name)
		if err == nil && reflect.DeepEqual(secret.Data, existingSec.Data) {
			return existingSec, nil
		}

		return nil, api.NewResponse(http.StatusConflict, api.ProvisionResponse{}, nil)
	}

	return nil, api.InternalServerError(err)
}

func (b *Broker) ensureTemplateInstance(impersonatedTemplateclient *templateclientset.Clientset, namespace string, instanceID string, template *templateapi.Template, secret *kapi.Secret, impersonate string, didWork *bool) (*templateapi.TemplateInstance, *api.Response) {
	templateInstance := &templateapi.TemplateInstance{
		ObjectMeta: kapi.ObjectMeta{Name: instanceID},
		Spec: templateapi.TemplateInstanceSpec{
			Template: *template,
			Secret:   kapi.LocalObjectReference{Name: secret.Name},
			Requestor: &templateapi.TemplateInstanceRequestor{
				Username: impersonate,
			},
		},
	}

	createdTemplateInstance, err := impersonatedTemplateclient.TemplateInstances(namespace).Create(templateInstance)
	if err == nil {
		*didWork = true
		return createdTemplateInstance, nil
	}

	if kerrors.IsAlreadyExists(err) {
		existingTemplateInstance, err := impersonatedTemplateclient.TemplateInstances(namespace).Get(templateInstance.Name)
		if err == nil && reflect.DeepEqual(templateInstance.Spec, existingTemplateInstance.Spec) {
			return existingTemplateInstance, nil
		}

		return nil, api.NewResponse(http.StatusConflict, api.ProvisionResponse{}, nil)
	}

	return nil, api.InternalServerError(err)
}

func (b *Broker) ensureBrokerTemplateInstanceUIDs(brokerTemplateInstance *templateapi.BrokerTemplateInstance, secret *kapi.Secret, templateInstance *templateapi.TemplateInstance, didWork *bool) (*templateapi.BrokerTemplateInstance, *api.Response) {
	brokerTemplateInstance.Spec.Secret.UID = secret.UID
	brokerTemplateInstance.Spec.TemplateInstance.UID = templateInstance.UID

	brokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Update(brokerTemplateInstance)
	if err == nil {
		*didWork = true
		return brokerTemplateInstance, nil
	}

	return nil, api.InternalServerError(err)
}

func (b *Broker) ensureBrokerTemplateInstance(namespace, instanceID string, didWork *bool) (*templateapi.BrokerTemplateInstance, *api.Response) {
	brokerTemplateInstance := &templateapi.BrokerTemplateInstance{
		ObjectMeta: kapi.ObjectMeta{Name: instanceID},
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

	newBrokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Create(brokerTemplateInstance)
	if err == nil {
		*didWork = true
		return newBrokerTemplateInstance, nil
	}

	if kerrors.IsAlreadyExists(err) {
		existingBrokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Get(brokerTemplateInstance.Name)
		if err == nil && reflect.DeepEqual(brokerTemplateInstance.Spec, existingBrokerTemplateInstance.Spec) {
			return existingBrokerTemplateInstance, nil
		}

		return nil, api.NewResponse(http.StatusConflict, api.ProvisionResponse{}, nil)
	}

	return nil, api.InternalServerError(err)
}

func (b *Broker) Provision(instanceID string, preq *api.ProvisionRequest) *api.Response {
	if errs := ValidateProvisionRequest(preq); len(errs) > 0 {
		return api.BadRequest(errs.ToAggregate())
	}

	namespace := preq.Parameters[templateapi.NamespaceParameterKey]
	impersonate := preq.Parameters[templateapi.RequesterUsernameParameterKey]

	impersonatedKC, _, impersonatedTemplateclient, err := b.getClientsForUsername(impersonate)
	if err != nil {
		return api.InternalServerError(err)
	}

	template, err := b.lister.GetTemplateByUID(preq.ServiceID)
	if err != nil {
		return api.BadRequest(err)
	}
	if template == nil {
		return api.BadRequest(kerrors.NewNotFound(templateapi.Resource("templates"), preq.ServiceID))
	}

	didWork := false

	brokerTemplateInstance, resp := b.ensureBrokerTemplateInstance(namespace, instanceID, &didWork)
	if resp != nil {
		return resp
	}

	secret, resp := b.ensureSecret(impersonatedKC, namespace, instanceID, preq, &didWork)
	if resp != nil {
		return resp
	}

	templateInstance, resp := b.ensureTemplateInstance(impersonatedTemplateclient, namespace, instanceID, template, secret, impersonate, &didWork)
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
