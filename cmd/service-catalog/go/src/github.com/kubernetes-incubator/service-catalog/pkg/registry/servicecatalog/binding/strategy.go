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
	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/golang/glog"
	sc "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	scv "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/validation"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
)

// NewScopeStrategy returns a new NamespaceScopedStrategy for bindings
func NewScopeStrategy() rest.NamespaceScopedStrategy {
	return bindingRESTStrategies
}

// implements interfaces RESTCreateStrategy, RESTUpdateStrategy, RESTDeleteStrategy,
// NamespaceScopedStrategy, and RESTGracefulDeleteStrategy
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
	_ rest.RESTCreateStrategy         = bindingRESTStrategies
	_ rest.RESTUpdateStrategy         = bindingRESTStrategies
	_ rest.RESTDeleteStrategy         = bindingRESTStrategies
	_ rest.RESTGracefulDeleteStrategy = bindingRESTStrategies

	bindingStatusUpdateStrategy = bindingStatusRESTStrategy{
		bindingRESTStrategies,
	}
	_ rest.RESTUpdateStrategy = bindingStatusUpdateStrategy
)

// Canonicalize does not transform a binding.
func (bindingRESTStrategy) Canonicalize(obj runtime.Object) {
	_, ok := obj.(*sc.ServiceBinding)
	if !ok {
		glog.Fatal("received a non-binding object to create")
	}
}

// NamespaceScoped returns true as bindings are scoped to a namespace.
func (bindingRESTStrategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate receives a the incoming ServiceBinding and clears it's
// Status. Status is not a user settable field.
func (bindingRESTStrategy) PrepareForCreate(ctx genericapirequest.Context, obj runtime.Object) {
	binding, ok := obj.(*sc.ServiceBinding)
	if !ok {
		glog.Fatal("received a non-binding object to create")
	}

	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		setServiceBindingUserInfo(binding, ctx)
	}

	// Creating a brand new object, thus it must have no
	// status. We can't fail here if they passed a status in, so
	// we just wipe it clean.
	binding.Status = sc.ServiceBindingStatus{
		UnbindStatus: sc.ServiceBindingUnbindStatusNotRequired,
	}
	// Fill in the first entry set to "creating"?
	binding.Status.Conditions = []sc.ServiceBindingCondition{}
	binding.Finalizers = []string{sc.FinalizerServiceCatalog}
	binding.Generation = 1
}

func (bindingRESTStrategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	return scv.ValidateServiceBinding(obj.(*sc.ServiceBinding))
}

func (bindingRESTStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (bindingRESTStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (bindingRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newServiceBinding, ok := new.(*sc.ServiceBinding)
	if !ok {
		glog.Fatal("received a non-binding object to update to")
	}
	oldServiceBinding, ok := old.(*sc.ServiceBinding)
	if !ok {
		glog.Fatal("received a non-binding object to update from")
	}
	newServiceBinding.Status = oldServiceBinding.Status

	// TODO: We currently don't handle any changes to the spec in the
	// reconciler. Once we do that, this check needs to be removed and
	// proper validation of allowed changes needs to be implemented in
	// ValidateUpdate. Also, the check for whether the generation needs
	// to be updated needs to be un-commented.
	newServiceBinding.Spec = oldServiceBinding.Spec

	// Spec updates bump the generation so that we can distinguish between
	// spec changes and other changes to the object.
	//
	// Note that since we do not currently handle any changes to the spec,
	// the generation will never be incremented
	if !apiequality.Semantic.DeepEqual(oldServiceBinding.Spec, newServiceBinding.Spec) {
		if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
			setServiceBindingUserInfo(newServiceBinding, ctx)
		}
		newServiceBinding.Generation = oldServiceBinding.Generation + 1
	}
}

func (bindingRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	newServiceBinding, ok := new.(*sc.ServiceBinding)
	if !ok {
		glog.Fatal("received a non-binding object to validate to")
	}
	oldServiceBinding, ok := old.(*sc.ServiceBinding)
	if !ok {
		glog.Fatal("received a non-binding object to validate from")
	}

	return scv.ValidateServiceBindingUpdate(newServiceBinding, oldServiceBinding)
}

// CheckGracefulDelete sets the UserInfo on the resource to that of the user that
// initiated the delete.
// Note that this is a hack way of setting the UserInfo. However, there is not
// currently any other mechanism in the Delete strategies for getting access to
// the resource being deleted and the context.
func (bindingRESTStrategy) CheckGracefulDelete(ctx genericapirequest.Context, obj runtime.Object, options *metav1.DeleteOptions) bool {
	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		serviceInstanceCredential, ok := obj.(*sc.ServiceBinding)
		if !ok {
			glog.Fatal("received a non-ServiceBinding object to delete")
		}
		setServiceBindingUserInfo(serviceInstanceCredential, ctx)
	}
	// Don't actually do graceful deletion. We are just using this strategy to set the user info prior to reconciling the delete.
	return false
}

func (bindingStatusRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newServiceBinding, ok := new.(*sc.ServiceBinding)
	if !ok {
		glog.Fatal("received a non-binding object to update to")
	}
	oldServiceBinding, ok := old.(*sc.ServiceBinding)
	if !ok {
		glog.Fatal("received a non-binding object to update from")
	}
	// status changes are not allowed to update spec
	newServiceBinding.Spec = oldServiceBinding.Spec
}

func (bindingStatusRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	newServiceBinding, ok := new.(*sc.ServiceBinding)
	if !ok {
		glog.Fatal("received a non-binding object to validate to")
	}
	oldServiceBinding, ok := old.(*sc.ServiceBinding)
	if !ok {
		glog.Fatal("received a non-binding object to validate from")
	}

	return scv.ValidateServiceBindingStatusUpdate(newServiceBinding, oldServiceBinding)
}

// setServiceBindingUserInfo injects user.Info from the request context
func setServiceBindingUserInfo(instanceCredential *sc.ServiceBinding, ctx genericapirequest.Context) {
	instanceCredential.Spec.UserInfo = nil
	if user, ok := genericapirequest.UserFrom(ctx); ok {
		instanceCredential.Spec.UserInfo = &sc.UserInfo{
			Username: user.GetName(),
			UID:      user.GetUID(),
			Groups:   user.GetGroups(),
		}
		if extra := user.GetExtra(); len(extra) > 0 {
			instanceCredential.Spec.UserInfo.Extra = map[string]sc.ExtraValue{}
			for k, v := range extra {
				instanceCredential.Spec.UserInfo.Extra[k] = sc.ExtraValue(v)
			}
		}
	}
}
