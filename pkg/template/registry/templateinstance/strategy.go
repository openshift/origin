package templateinstance

import (
	"errors"
	"fmt"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	templateapi "github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/origin/pkg/template/api/validation"
	userapi "github.com/openshift/origin/pkg/user/api"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/util/validation/field"
)

// templateInstanceStrategy implements behavior for Templates
type templateInstanceStrategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
	oc *client.Client
}

func NewStrategy(oc *client.Client) *templateInstanceStrategy {
	return &templateInstanceStrategy{kapi.Scheme, kapi.SimpleNameGenerator, oc}
}

// NamespaceScoped is true for templateinstances.
func (templateInstanceStrategy) NamespaceScoped() bool {
	return true
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (templateInstanceStrategy) PrepareForUpdate(ctx kapi.Context, obj, old runtime.Object) {
}

// Canonicalize normalizes the object after validation.
func (templateInstanceStrategy) Canonicalize(obj runtime.Object) {
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (templateInstanceStrategy) PrepareForCreate(ctx kapi.Context, obj runtime.Object) {
	templateInstance := obj.(*templateapi.TemplateInstance)

	if templateInstance.Spec.Requester == nil {
		user, _ := kapi.UserFrom(ctx)
		templateInstance.Spec.Requester = &templateapi.TemplateInstanceRequester{
			Username: user.GetName(),
		}
	}
}

// Validate validates a new templateinstance.
func (s *templateInstanceStrategy) Validate(ctx kapi.Context, obj runtime.Object) field.ErrorList {
	user, ok := kapi.UserFrom(ctx)
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
func (s *templateInstanceStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) field.ErrorList {
	user, ok := kapi.UserFrom(ctx)
	if !ok {
		return field.ErrorList{field.InternalError(field.NewPath(""), errors.New("user not found in context"))}
	}

	templateInstance := obj.(*templateapi.TemplateInstance)
	oldTemplateInstance := old.(*templateapi.TemplateInstance)
	allErrs := validation.ValidateTemplateInstanceUpdate(templateInstance, oldTemplateInstance)
	allErrs = append(allErrs, s.validateImpersonation(templateInstance, user)...)

	return allErrs
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(o runtime.Object) (labels.Set, fields.Set, error) {
			obj, ok := o.(*templateapi.TemplateInstance)
			if !ok {
				return nil, nil, fmt.Errorf("not a TemplateInstance")
			}
			return labels.Set(obj.Labels), SelectableFields(obj), nil
		},
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *templateapi.TemplateInstance) fields.Set {
	return templateapi.TemplateInstanceToSelectableFields(obj)
}

func (s *templateInstanceStrategy) validateImpersonation(templateInstance *templateapi.TemplateInstance, userinfo user.Info) field.ErrorList {
	if templateInstance.Spec.Requester == nil || templateInstance.Spec.Requester.Username == "" {
		return field.ErrorList{field.Required(field.NewPath("spec.requester.username"), "")}
	}

	if templateInstance.Spec.Requester.Username != userinfo.GetName() {
		sar := authorizationapi.AddUserToSAR(userinfo,
			&authorizationapi.SubjectAccessReview{
				Action: authorizationapi.Action{
					Verb:         "impersonate",
					Group:        userapi.GroupName,
					Resource:     authorizationapi.UserResource,
					ResourceName: templateInstance.Spec.Requester.Username,
				},
			})
		resp, err := s.oc.SubjectAccessReviews().Create(sar)
		if err != nil || resp == nil || !resp.Allowed {
			return field.ErrorList{field.Forbidden(field.NewPath("spec.impersonateUser"), "impersonation forbidden")}
		}
	}

	return nil
}
