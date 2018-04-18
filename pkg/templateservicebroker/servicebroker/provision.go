package servicebroker

import (
	"errors"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/golang/glog"

	authorizationv1 "k8s.io/api/authorization/v1"
	kapiv1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/util/retry"

	templateapiv1 "github.com/openshift/api/template/v1"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
	"github.com/openshift/origin/pkg/templateservicebroker/util"
)

const (
	invalidCustomLabelMsg = "Invalid Custom Label Parameter Value!"
)

// ensureSecret ensures the existence of a Secret object containing the template
// configuration parameters.
func (b *Broker) ensureSecret(u user.Info, namespace string, brokerTemplateInstance *templateapiv1.BrokerTemplateInstance, instanceID string, preq *api.ProvisionRequest, didWork *bool) (*kapiv1.Secret, *api.Response) {
	glog.V(4).Infof("Template service broker: ensureSecret")

	blockOwnerDeletion := true
	secret := &kapiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: instanceID,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         templateapiv1.SchemeGroupVersion.String(),
					Kind:               "BrokerTemplateInstance",
					Name:               brokerTemplateInstance.Name,
					UID:                brokerTemplateInstance.UID,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
		Data: map[string][]byte{},
	}

	for k, v := range preq.Parameters {
		secret.Data[k] = []byte(v)
	}

	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
		Namespace: namespace,
		Verb:      "create",
		Group:     kapiv1.GroupName,
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
		if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
			Namespace: namespace,
			Verb:      "get",
			Group:     kapiv1.GroupName,
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
func (b *Broker) ensureTemplateInstance(u user.Info, namespace string, brokerTemplateInstance *templateapiv1.BrokerTemplateInstance, instanceID string, template *templateapiv1.Template, secret *kapiv1.Secret, didWork *bool) (*templateapiv1.TemplateInstance, *api.Response) {
	glog.V(4).Infof("Template service broker: ensureTemplateInstance")

	extra := map[string]templateapiv1.ExtraValue{}
	for k, v := range u.GetExtra() {
		extra[k] = templateapiv1.ExtraValue(v)
	}

	blockOwnerDeletion := true
	templateInstance := &templateapiv1.TemplateInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: instanceID,
			Annotations: map[string]string{
				api.OpenServiceBrokerInstanceExternalID: instanceID,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         templateapiv1.SchemeGroupVersion.String(),
					Kind:               "BrokerTemplateInstance",
					Name:               brokerTemplateInstance.Name,
					UID:                brokerTemplateInstance.UID,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
		Spec: templateapiv1.TemplateInstanceSpec{
			Template: *template,
			Secret:   &kapiv1.LocalObjectReference{Name: secret.Name},
			Requester: &templateapiv1.TemplateInstanceRequester{
				Username: u.GetName(),
				UID:      u.GetUID(),
				Groups:   u.GetGroups(),
				Extra:    extra,
			},
		},
	}

	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
		Namespace: namespace,
		Verb:      "create",
		Group:     templateapiv1.GroupName,
		Resource:  "templateinstances",
		Name:      instanceID,
	}); err != nil {
		return nil, api.Forbidden(err)
	}

	createdTemplateInstance, err := b.templateclient.TemplateInstances(namespace).Create(templateInstance)
	if err == nil {
		*didWork = true
		return createdTemplateInstance, nil
	}

	if kerrors.IsAlreadyExists(err) {
		if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
			Namespace: namespace,
			Verb:      "get",
			Group:     templateapiv1.GroupName,
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
func (b *Broker) ensureBrokerTemplateInstanceUIDs(u user.Info, namespace string, brokerTemplateInstance *templateapiv1.BrokerTemplateInstance, secret *kapiv1.Secret, templateInstance *templateapiv1.TemplateInstance, didWork *bool) (*templateapiv1.BrokerTemplateInstance, *api.Response) {
	glog.V(4).Infof("Template service broker: ensureBrokerTemplateInstanceUIDs")

	// end users are not expected to have access to BrokerTemplateInstance
	// objects; SAR on the TemplateInstance instead.
	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
		Namespace: namespace,
		Verb:      "update",
		Group:     templateapiv1.GroupName,
		Resource:  "templateinstances",
		Name:      brokerTemplateInstance.Spec.TemplateInstance.Name,
	}); err != nil {
		return nil, api.Forbidden(err)
	}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		brokerTemplateInstance.Spec.Secret.UID = secret.UID
		brokerTemplateInstance.Spec.TemplateInstance.UID = templateInstance.UID

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
		*didWork = true
		return brokerTemplateInstance, nil
	case kerrors.IsConflict(err):
		return nil, api.NewResponse(http.StatusUnprocessableEntity, &api.ConcurrencyError, nil)
	}
	return nil, api.InternalServerError(err)
}

// ensureBrokerTemplateInstance ensures the existence of BrokerTemplateInstance
// object (records intent, globally maps instanceID to namespaced Secret and
// TemplateInstance objects).
func (b *Broker) ensureBrokerTemplateInstance(u user.Info, namespace, instanceID string, didWork *bool) (*templateapiv1.BrokerTemplateInstance, *api.Response) {
	glog.V(4).Infof("Template service broker: ensureBrokerTemplateInstance")

	brokerTemplateInstance := &templateapiv1.BrokerTemplateInstance{
		ObjectMeta: metav1.ObjectMeta{Name: instanceID},
		Spec: templateapiv1.BrokerTemplateInstanceSpec{
			TemplateInstance: kapiv1.ObjectReference{
				Kind:      "TemplateInstance",
				Namespace: namespace,
				Name:      instanceID,
			},
			Secret: kapiv1.ObjectReference{
				Kind:      "Secret",
				Namespace: namespace,
				Name:      instanceID,
			},
		},
	}

	// end users are not expected to have access to BrokerTemplateInstance
	// objects; SAR on the TemplateInstance instead.
	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
		Namespace: namespace,
		Verb:      "create",
		Group:     templateapiv1.GroupName,
		Resource:  "templateinstances",
		Name:      instanceID,
	}); err != nil {
		return nil, api.Forbidden(err)
	}

	newBrokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Create(brokerTemplateInstance)
	if err == nil {
		*didWork = true
		return newBrokerTemplateInstance, nil
	}

	if kerrors.IsAlreadyExists(err) {
		// end users are not expected to have access to BrokerTemplateInstance
		// objects; SAR on the TemplateInstance instead.
		if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
			Namespace: namespace,
			Verb:      "get",
			Group:     templateapiv1.GroupName,
			Resource:  "templateinstances",
			Name:      instanceID,
		}); err != nil {
			return nil, api.Forbidden(err)
		}

		existingBrokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Get(brokerTemplateInstance.Name, metav1.GetOptions{})
		if err == nil && reflect.DeepEqual(brokerTemplateInstance.Spec, existingBrokerTemplateInstance.Spec) {
			return existingBrokerTemplateInstance, nil
		}

		return nil, api.NewResponse(http.StatusConflict, api.ProvisionResponse{}, nil)
	}

	return nil, api.InternalServerError(err)
}

// ensureCustomLabel ensures that the Custom Label Parameter string 'param'
// gets parsed as labels and gets assigned to template's ObjectLabels[]
func (b *Broker) ensureCustomLabel(template *templateapiv1.Template, param string) *api.Response {
	customLabelParamStr := strings.TrimSpace(param)
	if len(customLabelParamStr) > 0 {
		// Validate and parse the custom label parameter.
		//  - Must be a valid k8s label ([a-zA-Z0-9], dot, dash, underscore)
		//  - Comma separated keys, or key=value pairs
		//		e.g.
		//			"k1=k2"
		//			"k1, k2, k3,..."
		//      "k1=v1, k2=v2, k3, k4, k5=v5, ... "
		paramRegex := regexp.MustCompile("^[-_. a-zA-Z0-9,=]*$")
		if paramRegex.MatchString(customLabelParamStr) {
			customLabels := strings.Split(customLabelParamStr, ",")
			for _, item := range customLabels {
				label := strings.TrimSpace(item)
				if len(label) > 0 {
					keyAndValue := strings.Split(label, "=")
					if len(keyAndValue[0]) > 0 {
						k := strings.TrimSpace(keyAndValue[0])
						v := ""
						if len(keyAndValue) >= 2 {
							v = strings.TrimSpace(keyAndValue[1])
						}
						// replace spaces with underscores
						k = strings.Replace(k, " ", "_", -1)
						v = strings.Replace(v, " ", "_", -1)
						// add to Template's ObjectLabels[]
						template.ObjectLabels[k] = v
					}
				}
			}
		} else {
			return api.InvalidCustomLabelParameter(errors.New(invalidCustomLabelMsg))
		}
	}
	return nil
}

// Provision instantiates a template from a ProvisionRequest, via the OpenShift
// TemplateInstance API.
func (b *Broker) Provision(u user.Info, instanceID string, preq *api.ProvisionRequest) *api.Response {
	glog.V(4).Infof("Template service broker: Provision: instanceID %s", instanceID)

	if errs := ValidateProvisionRequest(preq); len(errs) > 0 {
		return api.BadRequest(errs.ToAggregate())
	}

	namespace := preq.Context.Namespace

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
		return api.BadRequest(kerrors.NewNotFound(templateapiv1.Resource("templates"), preq.ServiceID))
	}
	if _, ok := b.templateNamespaces[template.Namespace]; !ok {
		return api.BadRequest(kerrors.NewNotFound(templateapiv1.Resource("templates"), preq.ServiceID))
	}

	// with groups in the user.Info vs. the username only form of auth, we can SAR for get access on template resources
	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
		Namespace: template.Namespace,
		Verb:      "get",
		Group:     templateapiv1.GroupName,
		Resource:  "templates",
		Name:      template.Name,
	}); err != nil {
		return api.Forbidden(err)
	}

	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
		Namespace: namespace,
		Verb:      "create",
		Group:     templateapiv1.GroupName,
		Resource:  "templateinstances",
		Name:      instanceID,
	}); err != nil {
		return api.Forbidden(err)
	}

	// Add Custom Labels
	resp := b.ensureCustomLabel(template, preq.Parameters[customLabelsParamName])
	if resp != nil {
		return resp
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

	brokerTemplateInstance, resp := b.ensureBrokerTemplateInstance(u, namespace, instanceID, &didWork)
	if resp != nil {
		return resp
	}

	// TODO remove this when https://github.com/kubernetes/kubernetes/issues/54940 is fixed
	time.Sleep(b.gcCreateDelay)

	secret, resp := b.ensureSecret(u, namespace, brokerTemplateInstance, instanceID, preq, &didWork)
	if resp != nil {
		return resp
	}

	templateInstance, resp := b.ensureTemplateInstance(u, namespace, brokerTemplateInstance, instanceID, template, secret, &didWork)
	if resp != nil {
		return resp
	}

	_, resp = b.ensureBrokerTemplateInstanceUIDs(u, namespace, brokerTemplateInstance, secret, templateInstance, &didWork)
	if resp != nil {
		return resp
	}

	if didWork {
		return api.NewResponse(http.StatusAccepted, api.ProvisionResponse{Operation: api.OperationProvisioning}, nil)
	}

	return api.NewResponse(http.StatusOK, api.ProvisionResponse{Operation: api.OperationProvisioning}, nil)
}
