/*
Copyright 2017 The Kubernetes Authors.

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

package binding

// this was copied from where else and edited to fit our objects

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/pkg/api"

	"github.com/golang/glog"
	sc "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	checksum "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/checksum/unversioned"
	scv "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/validation"
)

// NewScopeStrategy returns a new NamespaceScopedStrategy for bindings
func NewScopeStrategy() rest.NamespaceScopedStrategy {
	return bindingRESTStrategies
}

// implements interfaces RESTCreateStrategy, RESTUpdateStrategy, RESTDeleteStrategy,
// NamespaceScopedStrategy
type bindingRESTStrategy struct {
	runtime.ObjectTyper // inherit ObjectKinds method
	names.NameGenerator // GenerateName method for CreateStrategy
}

// implements interface RESTUpdateStrategy
type bindingStatusRESTStrategy struct {
	bindingRESTStrategy
}

var (
	bindingRESTStrategies = bindingRESTStrategy{
		// embeds to pull in existing code behavior from upstream

		ObjectTyper: api.Scheme,
		// use the generator from upstream k8s, or implement method
		// `GenerateName(base string) string`
		NameGenerator: names.SimpleNameGenerator,
	}
	_ rest.RESTCreateStrategy = bindingRESTStrategies
	_ rest.RESTUpdateStrategy = bindingRESTStrategies
	_ rest.RESTDeleteStrategy = bindingRESTStrategies

	bindingStatusUpdateStrategy = bindingStatusRESTStrategy{
		bindingRESTStrategies,
	}
	_ rest.RESTUpdateStrategy = bindingStatusUpdateStrategy
)

// Canonicalize does not transform a binding.
func (bindingRESTStrategy) Canonicalize(obj runtime.Object) {
	_, ok := obj.(*sc.Binding)
	if !ok {
		glog.Fatal("received a non-binding object to create")
	}
}

// NamespaceScoped returns false as bindings are not scoped to a namespace.
func (bindingRESTStrategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate receives a the incoming Binding and clears it's
// Status. Status is not a user settable field.
func (bindingRESTStrategy) PrepareForCreate(ctx genericapirequest.Context, obj runtime.Object) {
	binding, ok := obj.(*sc.Binding)
	if !ok {
		glog.Fatal("received a non-binding object to create")
	}
	// Is there anything to pull out of the context `ctx`?

	// Creating a brand new object, thus it must have no
	// status. We can't fail here if they passed a status in, so
	// we just wipe it clean.
	binding.Status = sc.BindingStatus{}
	// Fill in the first entry set to "creating"?
	binding.Status.Conditions = []sc.BindingCondition{}

	// TODO: Should we use a more specific string here?
	binding.Finalizers = []string{"kubernetes"}
}

func (bindingRESTStrategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	return scv.ValidateBinding(obj.(*sc.Binding))
}

func (bindingRESTStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (bindingRESTStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (bindingRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newBinding, ok := new.(*sc.Binding)
	if !ok {
		glog.Fatal("received a non-binding object to update to")
	}
	oldBinding, ok := old.(*sc.Binding)
	if !ok {
		glog.Fatal("received a non-binding object to update from")
	}
	newBinding.Spec = oldBinding.Spec
	newBinding.Status = oldBinding.Status
}

func (bindingRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	newBinding, ok := new.(*sc.Binding)
	if !ok {
		glog.Fatal("received a non-binding object to validate to")
	}
	oldBinding, ok := old.(*sc.Binding)
	if !ok {
		glog.Fatal("received a non-binding object to validate from")
	}

	return scv.ValidateBindingUpdate(newBinding, oldBinding)
}

func (bindingStatusRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newBinding, ok := new.(*sc.Binding)
	if !ok {
		glog.Fatal("received a non-binding object to update to")
	}
	oldBinding, ok := old.(*sc.Binding)
	if !ok {
		glog.Fatal("received a non-binding object to update from")
	}
	// status changes are not allowed to update spec
	newBinding.Spec = oldBinding.Spec

	foundReadyConditionTrue := false
	for _, condition := range newBinding.Status.Conditions {
		if condition.Type == sc.BindingConditionReady && condition.Status == sc.ConditionTrue {
			foundReadyConditionTrue = true
			break
		}
	}

	if foundReadyConditionTrue {
		glog.Infof("Found true ready condition for Binding %v/%v; updating checksum", newBinding.Namespace, newBinding.Name)
		// This status update has a true ready condition; update the checksum if necessary
		newBinding.Status.Checksum = func() *string {
			s := checksum.BindingSpecChecksum(newBinding.Spec)
			return &s
		}()
		return
	}

	// if the ready condition is not true, the value of the checksum should
	// not change.
	newBinding.Status.Checksum = oldBinding.Status.Checksum
}

func (bindingStatusRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	newBinding, ok := new.(*sc.Binding)
	if !ok {
		glog.Fatal("received a non-binding object to validate to")
	}
	oldBinding, ok := old.(*sc.Binding)
	if !ok {
		glog.Fatal("received a non-binding object to validate from")
	}

	return scv.ValidateBindingStatusUpdate(newBinding, oldBinding)
}
