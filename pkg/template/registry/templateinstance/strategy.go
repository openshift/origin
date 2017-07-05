package templateinstance

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/authorization"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/authorization/util"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/template/apis/template/validation"
)

// templateInstanceStrategy implements behavior for TemplateInstances
type templateInstanceStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
	kc kclientset.Interface
}

func NewStrategy(kc kclientset.Interface) *templateInstanceStrategy {
	return &templateInstanceStrategy{kapi.Scheme, names.SimpleNameGenerator, kc}
}

// NamespaceScoped is true for templateinstances.
func (templateInstanceStrategy) NamespaceScoped() bool {
	return true
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (templateInstanceStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	curr := obj.(*templateapi.TemplateInstance)
	prev := old.(*templateapi.TemplateInstance)

	curr.Status = prev.Status
}

// Canonicalize normalizes the object after validation.
func (templateInstanceStrategy) Canonicalize(obj runtime.Object) {
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (templateInstanceStrategy) PrepareForCreate(ctx apirequest.Context, obj runtime.Object) {
	templateInstance := obj.(*templateapi.TemplateInstance)

	if templateInstance.Spec.Requester == nil {
		user, _ := apirequest.UserFrom(ctx)
		templateInstance.Spec.Requester = &templateapi.TemplateInstanceRequester{
			Username: user.GetName(),
		}
	}

	templateInstance.Status = templateapi.TemplateInstanceStatus{}
}

// Validate validates a new templateinstance.
func (s *templateInstanceStrategy) Validate(ctx apirequest.Context, obj runtime.Object) field.ErrorList {
	user, ok := apirequest.UserFrom(ctx)
	if !ok {
		return field.ErrorList{field.InternalError(field.NewPath(""), errors.New("user not found in context"))}
	}

	templateInstance := obj.(*templateapi.TemplateInstance)
	allErrs := validation.ValidateTemplateInstance(templateInstance)
	allErrs = append(allErrs, s.validateImpersonation(templateInstance, user)...)

	return allErrs
}

// AllowCreateOnUpdate is false for templateinstances.
func (templateInstanceStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (templateInstanceStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end user.
func (s *templateInstanceStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	user, ok := apirequest.UserFrom(ctx)
	if !ok {
		return field.ErrorList{field.InternalError(field.NewPath(""), errors.New("user not found in context"))}
	}

	templateInstance := obj.(*templateapi.TemplateInstance)
	oldTemplateInstance := old.(*templateapi.TemplateInstance)
	allErrs := validation.ValidateTemplateInstanceUpdate(templateInstance, oldTemplateInstance)
	allErrs = append(allErrs, s.validateImpersonationUpdate(templateInstance, oldTemplateInstance, user)...)

	return allErrs
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// GetAttrs returns labels and fields of a given object for filtering purposes
func GetAttrs(o runtime.Object) (labels.Set, fields.Set, bool, error) {
	obj, ok := o.(*templateapi.TemplateInstance)
	if !ok {
		return nil, nil, false, fmt.Errorf("not a TemplateInstance")
	}
	return labels.Set(obj.Labels), SelectableFields(obj), obj.Initializers != nil, nil
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *templateapi.TemplateInstance) fields.Set {
	return templateapi.TemplateInstanceToSelectableFields(obj)
}

func (s *templateInstanceStrategy) validateImpersonationUpdate(templateInstance, oldTemplateInstance *templateapi.TemplateInstance, userinfo user.Info) field.ErrorList {
	if oadmission.IsOnlyMutatingGCFields(templateInstance, oldTemplateInstance) {
		return nil
	}

	return s.validateImpersonation(templateInstance, userinfo)
}

func (s *templateInstanceStrategy) validateImpersonation(templateInstance *templateapi.TemplateInstance, userinfo user.Info) field.ErrorList {
	if templateInstance.Spec.Requester == nil || templateInstance.Spec.Requester.Username == "" {
		return field.ErrorList{field.Required(field.NewPath("spec.requester.username"), "")}
	}

	if templateInstance.Spec.Requester.Username != userinfo.GetName() {
		if err := util.Authorize(s.kc.Authorization().SubjectAccessReviews(), userinfo, &authorization.ResourceAttributes{
			Namespace: templateInstance.Namespace,
			Verb:      "assign",
			Group:     templateapi.GroupName,
			Resource:  "templateinstances",
		}); err != nil {
			return field.ErrorList{field.Forbidden(field.NewPath("spec.requester.username"), "you do not have permission to set username")}
		}
	}

	return nil
}

type statusStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

var StatusStrategy = statusStrategy{kapi.Scheme, names.SimpleNameGenerator}

func (statusStrategy) NamespaceScoped() bool {
	return true
}

func (statusStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (statusStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (statusStrategy) PrepareForUpdate(ctx apirequest.Context, obj, old runtime.Object) {
	curr := obj.(*templateapi.TemplateInstance)
	prev := old.(*templateapi.TemplateInstance)

	curr.Spec = prev.Spec
}

func (statusStrategy) Canonicalize(obj runtime.Object) {
}

func (statusStrategy) ValidateUpdate(ctx apirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateTemplateInstanceUpdate(obj.(*templateapi.TemplateInstance), old.(*templateapi.TemplateInstance))
}
