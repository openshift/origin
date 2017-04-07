package servicebroker

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/openshift/origin/pkg/openservicebroker/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/selection"
)

// copied from vendor/k8s.io/kubernetes/pkg/kubelet/envvars/envvars.go
// TODO: when the API for returning information from the bind call is cleaned up
// and we're no longer temporarily using the environment variable style, remove
// this.
func makeEnvVariableName(str string) string {
	return strings.ToUpper(strings.Replace(str, "-", "_", -1))
}

func (b *Broker) getServices(impersonatedKC internalversion.ServicesGetter, namespace, instanceID string) (map[string]string, *api.Response) {
	requirement, _ := labels.NewRequirement(templateapi.TemplateInstanceLabel, selection.Equals, []string{instanceID})

	serviceList, err := impersonatedKC.Services(namespace).List(kapi.ListOptions{LabelSelector: labels.NewSelector().Add(*requirement)})
	if err != nil {
		if kerrors.IsForbidden(err) {
			return nil, api.Forbidden(err)
		}

		return nil, api.InternalServerError(err)
	}

	services := map[string]string{}
	for _, service := range serviceList.Items {
		if !kapi.IsServiceIPSet(&service) || len(service.Spec.Ports) == 0 {
			continue
		}

		name := makeEnvVariableName(service.Name) + "_SERVICE_HOST"
		services[name] = service.Spec.ClusterIP

		name = makeEnvVariableName(service.Name) + "_SERVICE_PORT"
		services[name] = strconv.Itoa(int(service.Spec.Ports[0].Port))

		for _, port := range service.Spec.Ports {
			if port.Name != "" {
				services[name+"_"+makeEnvVariableName(port.Name)] = strconv.Itoa(int(port.Port))
			}
		}
	}

	return services, nil
}

func (b *Broker) getSecrets(impersonatedKC internalversion.SecretsGetter, namespace, instanceID string) (map[string]string, *api.Response) {
	requirement, _ := labels.NewRequirement(templateapi.TemplateInstanceLabel, selection.Equals, []string{instanceID})

	secretList, err := impersonatedKC.Secrets(namespace).List(kapi.ListOptions{LabelSelector: labels.NewSelector().Add(*requirement)})
	if err != nil {
		if kerrors.IsForbidden(err) {
			return nil, api.Forbidden(err)
		}

		return nil, api.InternalServerError(err)
	}

	secrets := map[string]string{}
	for _, secret := range secretList.Items {
		if secret.Type != kapi.SecretTypeBasicAuth {
			continue
		}

		name := makeEnvVariableName(secret.Name + "_" + kapi.BasicAuthUsernameKey)
		secrets[name] = string(secret.Data[kapi.BasicAuthUsernameKey])

		name = makeEnvVariableName(secret.Name + "_" + kapi.BasicAuthPasswordKey)
		secrets[name] = string(secret.Data[kapi.BasicAuthPasswordKey])
	}

	return secrets, nil
}

func (b *Broker) Bind(instanceID, bindingID string, breq *api.BindRequest) *api.Response {
	if errs := ValidateBindRequest(breq); len(errs) > 0 {
		return api.BadRequest(errs.ToAggregate())
	}

	if len(breq.Parameters) != 1 {
		return api.BadRequest(errors.New("parameters not supported on bind"))
	}

	impersonate := breq.Parameters[templateapi.RequesterUsernameParameterKey]

	impersonatedKC, _, _, err := b.getClientsForUsername(impersonate)
	if err != nil {
		return api.InternalServerError(err)
	}

	brokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Get(instanceID)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return api.BadRequest(err)
		}

		return api.InternalServerError(err)
	}

	templateInstance, err := b.templateclient.TemplateInstances(brokerTemplateInstance.Spec.TemplateInstance.Namespace).Get(brokerTemplateInstance.Spec.TemplateInstance.Name)
	if err != nil {
		return api.InternalServerError(err)
	}
	if breq.ServiceID != string(templateInstance.Spec.Template.UID) {
		return api.BadRequest(errors.New("service_id does not match provisioned service"))
	}

	namespace := brokerTemplateInstance.Spec.TemplateInstance.Namespace

	services, resp := b.getServices(impersonatedKC.Core(), namespace, instanceID)
	if resp != nil {
		return resp
	}

	secrets, resp := b.getSecrets(impersonatedKC.Core(), namespace, instanceID)
	if resp != nil {
		return resp
	}

	// TODO: this API may not currently be considered stable.
	credentials := map[string]interface{}{}
	credentials["services"] = services
	credentials["secrets"] = secrets

	status := http.StatusCreated
	for _, id := range brokerTemplateInstance.Spec.BindingIDs {
		if id == bindingID {
			status = http.StatusOK
			break
		}
	}
	if status == http.StatusCreated {
		brokerTemplateInstance.Spec.BindingIDs = append(brokerTemplateInstance.Spec.BindingIDs, bindingID)
		brokerTemplateInstance, err = b.templateclient.BrokerTemplateInstances().Update(brokerTemplateInstance)
		if err != nil {
			return api.InternalServerError(err)
		}
	}

	return api.NewResponse(status, &api.BindResponse{Credentials: credentials}, nil)
}
