/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	"k8s.io/kubernetes/pkg/api/validation"
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
	return validation.ValidateSecurityContextConstraints(obj.(*api.SecurityContextConstraints))
}

func (strategy) ValidateUpdate(ctx genericapirequest.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateSecurityContextConstraintsUpdate(obj.(*api.SecurityContextConstraints), old.(*api.SecurityContextConstraints))
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	scc, ok := obj.(*api.SecurityContextConstraints)
	if !ok {
		return nil, nil, fmt.Errorf("not SecurityContextConstraints")
	}
	return labels.Set(scc.Labels), SelectableFields(scc), nil
}

// Matcher returns a generic matcher for a given label and field selector.
func Matcher(label labels.Selector, field fields.Selector) apistorage.SelectionPredicate {
	return apistorage.SelectionPredicate{
		Label: label,
		Field: field,
		GetAttrs: func(obj runtime.Object) (labels.Set, fields.Set, error) {
			scc, ok := obj.(*api.SecurityContextConstraints)
			if !ok {
				return nil, nil, fmt.Errorf("not a securitycontextconstraint")
			}
			return labels.Set(scc.Labels), SelectableFields(scc), nil
		},
	}
}

// SelectableFields returns a field set that can be used for filter selection
func SelectableFields(obj *api.SecurityContextConstraints) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&obj.ObjectMeta, true)
	return objectMetaFieldsSet
}
