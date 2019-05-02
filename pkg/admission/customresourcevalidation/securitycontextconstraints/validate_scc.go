package securitycontextconstraints

import (
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"

	securityv1 "github.com/openshift/api/security/v1"
	crvalidation "github.com/openshift/origin/pkg/admission/customresourcevalidation"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securityapiv1 "github.com/openshift/origin/pkg/security/apis/security/v1"
	securityvalidation "github.com/openshift/origin/pkg/security/apis/security/validation"
)

const ValidatePluginName = "security.openshift.io/ValidateSecurityContextConstraints"

func RegisterValidating(plugins *admission.Plugins) {
	plugins.Register(ValidatePluginName, func(config io.Reader) (admission.Interface, error) {
		return crvalidation.NewValidator(
			map[schema.GroupResource]bool{
				securityv1.GroupVersion.WithResource("securitycontextconstraints").GroupResource(): true,
			},
			map[schema.GroupVersionKind]crvalidation.ObjectValidator{
				securityv1.GroupVersion.WithKind("SecurityContextConstraints"): newValidater(),
			},
		)
	})
}

var _ crvalidation.ObjectValidator = &validateSCC{}

type validateSCC struct {
	scheme *runtime.Scheme
}

func newValidater() crvalidation.ObjectValidator {
	scheme := runtime.NewScheme()
	utilruntime.Must(securityv1.Install(scheme))
	utilruntime.Must(securityapiv1.Install(scheme))

	return &validateSCC{
		scheme: scheme,
	}
}

func (s *validateSCC) ValidateCreate(obj runtime.Object) field.ErrorList {
	scc, errs := s.toSCCInternal(obj)
	if len(errs) > 0 {
		return errs
	}

	return securityvalidation.ValidateSecurityContextConstraints(scc)
}

func (s *validateSCC) ValidateUpdate(obj runtime.Object, oldObj runtime.Object) field.ErrorList {
	newSCC, errs := s.toSCCInternal(obj)
	if len(errs) > 0 {
		return errs
	}
	oldSCC, errs := s.toSCCInternal(oldObj)
	if len(errs) > 0 {
		return errs
	}

	return securityvalidation.ValidateSecurityContextConstraintsUpdate(newSCC, oldSCC)
}

func (s *validateSCC) ValidateStatusUpdate(obj runtime.Object, oldObj runtime.Object) field.ErrorList {
	return nil
}

func (s *validateSCC) toSCCInternal(uncastObj runtime.Object) (*securityapi.SecurityContextConstraints, field.ErrorList) {
	if uncastObj == nil {
		return nil, nil
	}

	externalSCC, ok := uncastObj.(*securityv1.SecurityContextConstraints)
	if !ok {
		return nil, field.ErrorList{
			field.NotSupported(field.NewPath("kind"), fmt.Sprintf("%T", uncastObj), []string{"SecurityContextConstraints"}),
			field.NotSupported(field.NewPath("apiVersion"), fmt.Sprintf("%T", uncastObj), []string{securityv1.GroupVersion.String()}),
		}
	}

	internalSCC := &securityapi.SecurityContextConstraints{}
	if err := s.scheme.Convert(externalSCC, internalSCC, nil); err != nil {
		return nil, field.ErrorList{
			field.Invalid(field.NewPath(""), uncastObj, err.Error()),
		}
	}

	return internalSCC, nil
}
