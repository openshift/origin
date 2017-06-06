package servicebroker

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/util/jsonpath"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/authorization"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/authorization/util"
	"github.com/openshift/origin/pkg/openservicebroker/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

func evaluateJSONPathExpression(obj interface{}, annotation, expression string, base64encode bool) (string, error) {
	var s []string

	j := jsonpath.New("templateservicebroker")
	err := j.Parse(expression)
	if err != nil {
		return "", fmt.Errorf("failed to parse annotation %s: %v", annotation, err)
	}

	results, err := j.FindResults(obj)
	if err != nil {
		return "", fmt.Errorf("FindResults failed on annotation %s: %v", annotation, err)
	}

	for _, r := range results {
		// we don't permit individual JSONPath expressions which return multiple
		// objects as we haven't decided how these should be output
		if len(r) != 1 {
			return "", fmt.Errorf("%d JSONPath results found on annotation %s", len(r), annotation)
		}

		result := r[0]

		// give one shot at dereferencing an interface/pointer.
		switch result.Kind() {
		case reflect.Interface, reflect.Ptr:
			if result.IsNil() {
				return "", fmt.Errorf("nil kind %s found in JSONPath result on annotation %s", result.Kind(), annotation)
			}
			result = result.Elem()
		}

		switch result.Kind() {
		// all the simple types
		case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
			reflect.Int64, reflect.Uint, reflect.Uint16, reflect.Uint32,
			reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64,
			reflect.Complex64, reflect.Complex128, reflect.String:
			s = append(s, fmt.Sprint(result.Interface()))
			continue

		// for now, the only complex type we permit is []byte
		case reflect.Slice:
			if result.Type().Elem().Kind() == reflect.Uint8 {
				if !base64encode {
					// convert from []byte to string.  Potentially lossy but
					// "friendly".  Per the golang spec, invalid UTF-8 sequences
					// will be replaced by 0xFFFD, the Unicode replacement
					// character
					s = append(s, string(result.Bytes()))

				} else {
					b := &bytes.Buffer{}
					w := base64.NewEncoder(base64.StdEncoding, b)
					w.Write(result.Bytes())
					w.Close()
					s = append(s, b.String())
				}
				continue
			}
		}

		return "", fmt.Errorf("unrepresentable kind %s found in JSONPath result on annotation %s", result.Kind(), annotation)
	}

	return strings.Join(s, ""), nil
}

// updateCredentialsForObject evaluates all ExposeAnnotationPrefix and
// Base64ExposeAnnotationPrefix JSONPath annotations on a given object, updating
// credentials as it goes.  Important: the object must be external ("v1") rather
// than internal so that lower-case JSONPath expressons (e.g. "{.spec}")
// evaluate correctly.
func updateCredentialsForObject(credentials map[string]interface{}, obj runtime.Object) error {
	meta, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	for k, v := range meta.GetAnnotations() {
		var prefix string

		for _, p := range []string{templateapi.ExposeAnnotationPrefix, templateapi.Base64ExposeAnnotationPrefix} {
			if strings.HasPrefix(k, p) {
				prefix = p
				break
			}
		}

		if prefix != "" && len(k) > len(prefix) {
			result, err := evaluateJSONPathExpression(obj, k, v, prefix == templateapi.Base64ExposeAnnotationPrefix)
			if err != nil {
				return err
			}
			credentials[k[len(prefix):]] = result
		}
	}

	return nil
}

// updateCredentials lists objects of a particular type created by a given
// template in its namespace and calls updateCredentialsForObject for each.
// TODO: handle objects created in other namespaces as well.
func (b *Broker) updateCredentials(u user.Info, namespace, instanceID, group, resource string, credentials map[string]interface{}, lister func(metav1.ListOptions) (runtime.Object, error)) *api.Response {
	glog.V(4).Infof("Template service broker: updateCredentials: group %s, resource: %s", group, resource)

	requirement, _ := labels.NewRequirement(templateapi.TemplateInstanceLabel, selection.Equals, []string{instanceID})

	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
		Namespace: namespace,
		Verb:      "list",
		Group:     group,
		Resource:  resource,
	}); err != nil {
		return api.Forbidden(err)
	}

	list, err := lister(metav1.ListOptions{LabelSelector: labels.NewSelector().Add(*requirement).String()})
	if err != nil {
		if kerrors.IsForbidden(err) {
			return api.Forbidden(err)
		}

		return api.InternalServerError(err)
	}

	err = meta.EachListItem(list, func(obj runtime.Object) error {
		return updateCredentialsForObject(credentials, obj)
	})

	if err != nil {
		return api.InternalServerError(err)
	}

	return nil

}

// Bind returns the secrets and services from a provisioned template.
func (b *Broker) Bind(instanceID, bindingID string, breq *api.BindRequest) *api.Response {
	glog.V(4).Infof("Template service broker: Bind: instanceID %s, bindingID %s", instanceID, bindingID)

	if errs := ValidateBindRequest(breq); len(errs) > 0 {
		return api.BadRequest(errs.ToAggregate())
	}

	if len(breq.Parameters) != 1 {
		return api.BadRequest(errors.New("parameters not supported on bind"))
	}

	impersonate := breq.Parameters[templateapi.RequesterUsernameParameterKey]
	u := &user.DefaultInfo{Name: impersonate}

	brokerTemplateInstance, err := b.templateclient.BrokerTemplateInstances().Get(instanceID, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return api.BadRequest(err)
		}

		return api.InternalServerError(err)
	}

	namespace := brokerTemplateInstance.Spec.TemplateInstance.Namespace

	// since we can, cross-check breq.ServiceID and
	// templateInstance.Spec.Template.UID.

	if err := util.Authorize(b.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
		Namespace: namespace,
		Verb:      "get",
		Group:     templateapi.GroupName,
		Resource:  "templateinstances",
	}); err != nil {
		return api.Forbidden(err)
	}

	templateInstance, err := b.templateclient.TemplateInstances(namespace).Get(brokerTemplateInstance.Spec.TemplateInstance.Name, metav1.GetOptions{})
	if err != nil {
		return api.InternalServerError(err)
	}
	if breq.ServiceID != string(templateInstance.Spec.Template.UID) {
		return api.BadRequest(errors.New("service_id does not match provisioned service"))
	}

	credentials := map[string]interface{}{}

	resp := b.updateCredentials(u, namespace, instanceID, kapi.GroupName, "configmaps", credentials, func(o metav1.ListOptions) (runtime.Object, error) {
		return b.extkc.Core().ConfigMaps(namespace).List(o)
	})
	if resp != nil {
		return resp
	}

	resp = b.updateCredentials(u, namespace, instanceID, kapi.GroupName, "secrets", credentials, func(o metav1.ListOptions) (runtime.Object, error) {
		return b.extkc.Core().Secrets(namespace).List(o)
	})
	if resp != nil {
		return resp
	}

	resp = b.updateCredentials(u, namespace, instanceID, kapi.GroupName, "services", credentials, func(o metav1.ListOptions) (runtime.Object, error) {
		return b.extkc.Core().Services(namespace).List(o)
	})
	if resp != nil {
		return resp
	}

	resp = b.updateCredentials(u, namespace, instanceID, routeapi.GroupName, "routes", credentials, func(o metav1.ListOptions) (runtime.Object, error) {
		return b.extrouteclient.Routes(namespace).List(o)
	})
	if resp != nil {
		return resp
	}

	// The OSB API requires this function to be idempotent (restartable).  If
	// any actual change was made, per the spec, StatusCreated is returned, else
	// StatusOK.

	status := http.StatusCreated
	for _, id := range brokerTemplateInstance.Spec.BindingIDs {
		if id == bindingID {
			status = http.StatusOK
			break
		}
	}
	if status == http.StatusCreated { // binding not found; create it
		brokerTemplateInstance.Spec.BindingIDs = append(brokerTemplateInstance.Spec.BindingIDs, bindingID)
		brokerTemplateInstance, err = b.templateclient.BrokerTemplateInstances().Update(brokerTemplateInstance)
		if err != nil {
			return api.InternalServerError(err)
		}
	}

	return api.NewResponse(status, &api.BindResponse{Credentials: credentials}, nil)
}
