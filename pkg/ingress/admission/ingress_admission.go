// This plugin supplements upstream Ingress admission validation
// It takes care of current Openshift specific constraints on Ingress resources
package admission

import (
	"fmt"
	"io"
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	kextensions "k8s.io/kubernetes/pkg/apis/extensions"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configlatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/ingress/admission/apis/ingressadmission"
)

const (
	IngressAdmission = "openshift.io/IngressAdmission"
)

func Register(plugins *admission.Plugins) {
	plugins.Register(IngressAdmission,
		func(config io.Reader) (admission.Interface, error) {
			pluginConfig, err := readConfig(config)
			if err != nil {
				return nil, err
			}
			return NewIngressAdmission(pluginConfig), nil
		})
}

type ingressAdmission struct {
	*admission.Handler
	config     *ingressadmission.IngressAdmissionConfig
	authorizer authorizer.Authorizer
}

var _ = oadmission.WantsAuthorizer(&ingressAdmission{})

func NewIngressAdmission(config *ingressadmission.IngressAdmissionConfig) *ingressAdmission {
	return &ingressAdmission{
		Handler: admission.NewHandler(admission.Create, admission.Update),
		config:  config,
	}
}

func readConfig(reader io.Reader) (*ingressadmission.IngressAdmissionConfig, error) {
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
	config, ok := obj.(*ingressadmission.IngressAdmissionConfig)
	if !ok {
		return nil, fmt.Errorf("unexpected config object: %#v", obj)
	}
	// No validation needed since config is just list of strings
	return config, nil
}

func (r *ingressAdmission) SetAuthorizer(a authorizer.Authorizer) {
	r.authorizer = a
}

func (r *ingressAdmission) ValidateInitialization() error {
	if r.authorizer == nil {
		return fmt.Errorf("%s needs an Openshift Authorizer", IngressAdmission)
	}
	return nil
}

func (r *ingressAdmission) Admit(a admission.Attributes) error {
	if a.GetResource().GroupResource() == kextensions.Resource("ingresses") {
		switch a.GetOperation() {
		case admission.Create:
			if ingress, ok := a.GetObject().(*kextensions.Ingress); ok {
				// if any rules have a host, check whether the user has permission to set them
				for i, rule := range ingress.Spec.Rules {
					if len(rule.Host) > 0 {
						attr := authorizer.AttributesRecord{
							User:            a.GetUserInfo(),
							Verb:            "create",
							Namespace:       a.GetNamespace(),
							Resource:        "routes",
							Subresource:     "custom-host",
							APIGroup:        "route.openshift.io",
							ResourceRequest: true,
						}
						kind := schema.GroupKind{Group: a.GetResource().Group, Kind: a.GetResource().Resource}
						authorized, _, err := r.authorizer.Authorize(attr)
						if err != nil {
							return errors.NewInvalid(kind, ingress.Name, field.ErrorList{field.InternalError(field.NewPath("spec", "rules").Index(i), err)})
						}
						if authorized != authorizer.DecisionAllow {
							return errors.NewInvalid(kind, ingress.Name, field.ErrorList{field.Forbidden(field.NewPath("spec", "rules").Index(i), "you do not have permission to set host fields in ingress rules")})
						}
						break
					}
				}
			}
		case admission.Update:
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
					attr := authorizer.AttributesRecord{
						User:            a.GetUserInfo(),
						Verb:            "update",
						Namespace:       a.GetNamespace(),
						Name:            a.GetName(),
						Resource:        "routes",
						Subresource:     "custom-host",
						APIGroup:        "route.openshift.io",
						ResourceRequest: true,
					}
					kind := schema.GroupKind{Group: a.GetResource().Group, Kind: a.GetResource().Resource}
					authorized, _, err := r.authorizer.Authorize(attr)
					if err != nil {
						return errors.NewInvalid(kind, newIngress.Name, field.ErrorList{field.InternalError(field.NewPath("spec", "rules"), err)})
					}
					if authorized == authorizer.DecisionAllow {
						return nil
					}
					return fmt.Errorf("cannot change hostname")
				}
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
