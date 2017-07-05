package securitycontextconstraints

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	apistorage "k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/pkg/api"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	"github.com/openshift/origin/pkg/security/apis/security/validation"
)

// strategy implements behavior for SecurityContextConstraints objects
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy is the default logic that applies when creating and updating ServiceAccount
// objects via the REST API.
var Strategy = strategy{api.Scheme, names.SimpleNameGenerator}

var _ = rest.RESTCreateStrategy(Strategy)

var _ = rest.RESTUpdateStrategy(Strategy)

func (strategy) NamespaceScoped() bool {
	return false
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) AllowUnconditionalUpdate() bool {
	return true
}

func (strategy) PrepareForCreate(_ genericapirequest.Context, obj runtime.Object) {
}

func (strategy) PrepareForUpdate(_ genericapirequest.Context, obj, old runtime.Object) {
}

func (strategy) Canonicalize(obj runtime.Object) {
}

func (strategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	return validation.ValidateSecurityContextConstraints(obj.(*securityapi.SecurityContextConstraints))
}

func (strategy) ValidateUpdate(ctx genericapirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateSecurityContextConstraintsUpdate(obj.(*securityapi.SecurityContextConstraints), old.(*securityapi.SecurityContextConstraints))
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, bool, error) {
	scc, ok := obj.(*securityapi.SecurityContextConstraints)
	if !ok {
		return nil, nil, false, fmt.Errorf("not SecurityContextConstraints")
	}
	return labels.Set(scc.Labels), SelectableFields(scc), scc.Initializers != nil, nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) apistorage.SelectionPredicate {
	return apistorage.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(obj runtime.Object) (labels.Set, fields.Set, bool, error) {
			scc, ok := obj.(*securityapi.SecurityContextConstraints)
			if !ok {
				return nil, nil, false, fmt.Errorf("not a securitycontextconstraint")
			}
			return labels.Set(scc.Labels), SelectableFields(scc), scc.Initializers != nil, nil
		},
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *securityapi.SecurityContextConstraints) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&obj.ObjectMeta, true)
	return objectMetaFieldsSet
}
