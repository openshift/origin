package templateinstance

import (
	"context"
	"errors"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage/names"
	authorizationclient "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	rbacregistry "k8s.io/kubernetes/pkg/registry/rbac"

	"github.com/openshift/origin/pkg/authorization/util"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/template/apis/template/validation"
)

// templateInstanceStrategy implements behavior for TemplateInstances
type templateInstanceStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
	authorizationClient authorizationclient.AuthorizationV1Interface
}

func NewStrategy(authorizationClient authorizationclient.AuthorizationV1Interface) *templateInstanceStrategy {
	return &templateInstanceStrategy{legacyscheme.Scheme, names.SimpleNameGenerator, authorizationClient}
}

// NamespaceScoped is true for templateinstances.
func (templateInstanceStrategy) NamespaceScoped() bool {
	return true
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (templateInstanceStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	curr := obj.(*templateapi.TemplateInstance)
	prev := old.(*templateapi.TemplateInstance)

	curr.Status = prev.Status
}

// Canonicalize normalizes the object after validation.
func (templateInstanceStrategy) Canonicalize(obj runtime.Object) {
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (templateInstanceStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	templateInstance := obj.(*templateapi.TemplateInstance)

	// if request not set, pull from context; note: the requester can be set via the service catalog
	// propagating the user information via the openservicebroker origination api header on
	// calls to the TSB endpoints (i.e. the Provision call)
	if templateInstance.Spec.Requester == nil {

		if user, ok := apirequest.UserFrom(ctx); ok {
			templateReq := convertUserToTemplateInstanceRequester(user)
			templateInstance.Spec.Requester = &templateReq
		}
	}

	templateInstance.Status = templateapi.TemplateInstanceStatus{}
}

// Validate validates a new templateinstance.
func (s *templateInstanceStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
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
func (s *templateInstanceStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	user, ok := apirequest.UserFrom(ctx)
	if !ok {
		return field.ErrorList{field.InternalError(field.NewPath(""), errors.New("user not found in context"))}
	}

	// Decode Spec.Template.Objects on both obj and old to Unstructureds.  This
	// allows detectection of at least some cases where the Objects are
	// semantically identical, but the serialisations have been jumbled up.  One
	// place where this happens is in the garbage collector, which uses
	// Unstructureds via the dynamic client.

	if obj == nil {
		return field.ErrorList{field.InternalError(field.NewPath(""), errors.New("input object is nil"))}
	}
	templateInstanceCopy := obj.DeepCopyObject()
	templateInstance := templateInstanceCopy.(*templateapi.TemplateInstance)

	errs := runtime.DecodeList(templateInstance.Spec.Template.Objects, unstructured.UnstructuredJSONScheme)
	if len(errs) != 0 {
		return field.ErrorList{field.InternalError(field.NewPath(""), kutilerrors.NewAggregate(errs))}
	}

	if old == nil {
		return field.ErrorList{field.InternalError(field.NewPath(""), errors.New("input object is nil"))}
	}
	oldTemplateInstanceCopy := old.DeepCopyObject()
	oldTemplateInstance := oldTemplateInstanceCopy.(*templateapi.TemplateInstance)

	errs = runtime.DecodeList(oldTemplateInstance.Spec.Template.Objects, unstructured.UnstructuredJSONScheme)
	if len(errs) != 0 {
		return field.ErrorList{field.InternalError(field.NewPath(""), kutilerrors.NewAggregate(errs))}
	}

	allErrs := validation.ValidateTemplateInstanceUpdate(templateInstance, oldTemplateInstance)
	allErrs = append(allErrs, s.validateImpersonationUpdate(templateInstance, oldTemplateInstance, user)...)

	return allErrs
}

func (s *templateInstanceStrategy) validateImpersonationUpdate(templateInstance, oldTemplateInstance *templateapi.TemplateInstance, userinfo user.Info) field.ErrorList {
	if rbacregistry.IsOnlyMutatingGCFields(templateInstance, oldTemplateInstance, kapihelper.Semantic) {
		return nil
	}

	return s.validateImpersonation(templateInstance, userinfo)
}

func (s *templateInstanceStrategy) validateImpersonation(templateInstance *templateapi.TemplateInstance, userinfo user.Info) field.ErrorList {
	if templateInstance.Spec.Requester == nil || templateInstance.Spec.Requester.Username == "" {
		return field.ErrorList{field.Required(field.NewPath("spec.requester.username"), "")}
	}

	if templateInstance.Spec.Requester.Username != userinfo.GetName() {
		if err := util.Authorize(s.authorizationClient.SubjectAccessReviews(), userinfo, &authorizationv1.ResourceAttributes{
			Namespace: templateInstance.Namespace,
			Verb:      "assign",
			Group:     templateapi.GroupName,
			Resource:  "templateinstances",
			Name:      templateInstance.Name,
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

var StatusStrategy = statusStrategy{legacyscheme.Scheme, names.SimpleNameGenerator}

func (statusStrategy) NamespaceScoped() bool {
	return true
}

func (statusStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (statusStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (statusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	curr := obj.(*templateapi.TemplateInstance)
	prev := old.(*templateapi.TemplateInstance)

	curr.Spec = prev.Spec
}

func (statusStrategy) Canonicalize(obj runtime.Object) {
}

func (statusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateTemplateInstanceUpdate(obj.(*templateapi.TemplateInstance), old.(*templateapi.TemplateInstance))
}

// convertUserToTemplateInstanceRequester copies analogous fields from user.Info to TemplateInstanceRequester
func convertUserToTemplateInstanceRequester(u user.Info) templateapi.TemplateInstanceRequester {
	templatereq := templateapi.TemplateInstanceRequester{}

	if u != nil {
		extra := map[string]templateapi.ExtraValue{}
		if u.GetExtra() != nil {
			for k, v := range u.GetExtra() {
				extra[k] = templateapi.ExtraValue(v)
			}
		}

		templatereq.Username = u.GetName()
		templatereq.UID = u.GetUID()
		templatereq.Groups = u.GetGroups()
		templatereq.Extra = extra
	}

	return templatereq
}
