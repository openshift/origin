// This plugin supplements upstream Ingress admission validation
// It takes care of current Openshift specific constraints on Ingress resources
package admission

import (
	"fmt"
	"io"
	"reflect"

	"k8s.io/client-go/pkg/util/sets"
	kadmission "k8s.io/kubernetes/pkg/admission"
	kextensions "k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/ingress/admission/api"
)

const (
	IngressAdmission = "openshift.io/IngressAdmission"
)

func init() {
	kadmission.RegisterPlugin(IngressAdmission, func(clien clientset.Interface, config io.Reader) (kadmission.Interface, error) {
		pluginConfig, err := readConfig(config)
		if err != nil {
			return nil, err
		}
		return NewIngressAdmission(pluginConfig), nil
	})
}

type ingressAdmission struct {
	*kadmission.Handler
	config *api.IngressAdmissionConfig
}

func NewIngressAdmission(config *api.IngressAdmissionConfig) *ingressAdmission {
	return &ingressAdmission{
		Handler: kadmission.NewHandler(kadmission.Create, kadmission.Update),
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

func (r *ingressAdmission) Admit(a kadmission.Attributes) error {
	if a.GetResource().GroupResource() == kextensions.Resource("ingresses") && a.GetOperation() == kadmission.Update {
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
