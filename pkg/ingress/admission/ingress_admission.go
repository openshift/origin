// This plugin supplements upstream Ingress admission validation
// It takes care of current Openshift specific constraints on Ingress resources
package admission

import (
	"fmt"
	"io"
	"reflect"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	kextensions "k8s.io/kubernetes/pkg/apis/extensions"

	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/ingress/admission/api"
)

const (
	IngressAdmission = "openshift.io/IngressAdmission"
)

func init() {
	admission.RegisterPlugin(IngressAdmission, func(config io.Reader) (admission.Interface, error) {
		pluginConfig, err := readConfig(config)
		if err != nil {
			return nil, err
		}
		return NewIngressAdmission(pluginConfig), nil
	})
}

type ingressAdmission struct {
	*admission.Handler
	config *api.IngressAdmissionConfig
}

func NewIngressAdmission(config *api.IngressAdmissionConfig) *ingressAdmission {
	return &ingressAdmission{
		Handler: admission.NewHandler(admission.Create, admission.Update),
		config:  config,
	}
}

func readConfig(reader io.Reader) (*api.IngressAdmissionConfig, error) {
	if reader == nil || reflect.ValueOf(reader).IsNil() {
		return nil, nil
	}
	obj, err := configlatest.ReadYAML(reader)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	config, ok := obj.(*api.IngressAdmissionConfig)
	if !ok {
		return nil, fmt.Errorf("unexpected config object: %#v", obj)
	}
	// No validation needed since config is just list of strings
	return config, nil
}

func (r *ingressAdmission) Admit(a admission.Attributes) error {
	if a.GetResource().GroupResource() == kextensions.Resource("ingresses") && a.GetOperation() == admission.Update {
		if r.config == nil || r.config.AllowHostnameChanges == false {
			oldIngress, ok := a.GetOldObject().(*kextensions.Ingress)
			if !ok {
				return nil
			}
			newIngress, ok := a.GetObject().(*kextensions.Ingress)
			if !ok {
				return nil
			}
			if !haveHostnamesChanged(oldIngress, newIngress) {
				return fmt.Errorf("cannot change hostname")
			}
		}
	}
	return nil
}

func haveHostnamesChanged(oldIngress, newIngress *kextensions.Ingress) bool {
	hostnameSet := sets.NewString()
	for _, element := range oldIngress.Spec.Rules {
		hostnameSet.Insert(element.Host)
	}

	for _, element := range newIngress.Spec.Rules {
		if present := hostnameSet.Has(element.Host); !present {
			return false
		}
	}

	return true
}
