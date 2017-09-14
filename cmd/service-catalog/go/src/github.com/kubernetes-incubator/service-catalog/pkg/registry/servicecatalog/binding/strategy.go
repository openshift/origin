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
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/pkg/api"

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
	_, ok := obj.(*sc.ServiceInstanceCredential)
	if !ok {
		glog.Fatal("received a non-binding object to create")
	}
}

// NamespaceScoped returns false as bindings are not scoped to a namespace.
func (bindingRESTStrategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate receives a the incoming ServiceInstanceCredential and clears it's
// Status. Status is not a user settable field.
func (bindingRESTStrategy) PrepareForCreate(ctx genericapirequest.Context, obj runtime.Object) {
	binding, ok := obj.(*sc.ServiceInstanceCredential)
	if !ok {
		glog.Fatal("received a non-binding object to create")
	}

	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		setServiceInstanceCredentialUserInfo(binding, ctx)
	}

	// Creating a brand new object, thus it must have no
	// status. We can't fail here if they passed a status in, so
	// we just wipe it clean.
	binding.Status = sc.ServiceInstanceCredentialStatus{}
	// Fill in the first entry set to "creating"?
	binding.Status.Conditions = []sc.ServiceInstanceCredentialCondition{}
	binding.Finalizers = []string{sc.FinalizerServiceCatalog}
	binding.Generation = 1
}

func (bindingRESTStrategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	return scv.ValidateServiceInstanceCredential(obj.(*sc.ServiceInstanceCredential))
}

func (bindingRESTStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (bindingRESTStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (bindingRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newServiceInstanceCredential, ok := new.(*sc.ServiceInstanceCredential)
	if !ok {
		glog.Fatal("received a non-binding object to update to")
	}
	oldServiceInstanceCredential, ok := old.(*sc.ServiceInstanceCredential)
	if !ok {
		glog.Fatal("received a non-binding object to update from")
	}
	newServiceInstanceCredential.Status = oldServiceInstanceCredential.Status

	// TODO: We currently don't handle any changes to the spec in the
	// reconciler. Once we do that, this check needs to be removed and
	// proper validation of allowed changes needs to be implemented in
	// ValidateUpdate. Also, the check for whether the generation needs
	// to be updated needs to be un-commented.
	newServiceInstanceCredential.Spec = oldServiceInstanceCredential.Spec

	// Spec updates bump the generation so that we can distinguish between
	// spec changes and other changes to the object.
	//
	// Note that since we do not currently handle any changes to the spec,
	// the generation will never be incremented
	if !apiequality.Semantic.DeepEqual(oldServiceInstanceCredential.Spec, newServiceInstanceCredential.Spec) {
		if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
			setServiceInstanceCredentialUserInfo(newServiceInstanceCredential, ctx)
		}
		newServiceInstanceCredential.Generation = oldServiceInstanceCredential.Generation + 1
	}
}

func (bindingRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	newServiceInstanceCredential, ok := new.(*sc.ServiceInstanceCredential)
	if !ok {
		glog.Fatal("received a non-binding object to validate to")
	}
	oldServiceInstanceCredential, ok := old.(*sc.ServiceInstanceCredential)
	if !ok {
		glog.Fatal("received a non-binding object to validate from")
	}

	return scv.ValidateServiceInstanceCredentialUpdate(newServiceInstanceCredential, oldServiceInstanceCredential)
}

// CheckGracefulDelete sets the UserInfo on the resource to that of the user that
// initiated the delete.
// Note that this is a hack way of setting the UserInfo. However, there is not
// currently any other mechanism in the Delete strategies for getting access to
// the resource being deleted and the context.
func (bindingRESTStrategy) CheckGracefulDelete(ctx genericapirequest.Context, obj runtime.Object, options *metav1.DeleteOptions) bool {
	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		serviceInstanceCredential, ok := obj.(*sc.ServiceInstanceCredential)
		if !ok {
			glog.Fatal("received a non-ServiceInstanceCredential object to delete")
		}
		setServiceInstanceCredentialUserInfo(serviceInstanceCredential, ctx)
	}
	// Don't actually do graceful deletion. We are just using this strategy to set the user info prior to reconciling the delete.
	return false
}

func (bindingStatusRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newServiceInstanceCredential, ok := new.(*sc.ServiceInstanceCredential)
	if !ok {
		glog.Fatal("received a non-binding object to update to")
	}
	oldServiceInstanceCredential, ok := old.(*sc.ServiceInstanceCredential)
	if !ok {
		glog.Fatal("received a non-binding object to update from")
	}
	// status changes are not allowed to update spec
	newServiceInstanceCredential.Spec = oldServiceInstanceCredential.Spec
}

func (bindingStatusRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	newServiceInstanceCredential, ok := new.(*sc.ServiceInstanceCredential)
	if !ok {
		glog.Fatal("received a non-binding object to validate to")
	}
	oldServiceInstanceCredential, ok := old.(*sc.ServiceInstanceCredential)
	if !ok {
		glog.Fatal("received a non-binding object to validate from")
	}

	return scv.ValidateServiceInstanceCredentialStatusUpdate(newServiceInstanceCredential, oldServiceInstanceCredential)
}

// setServiceInstanceCredentialUserInfo injects user.Info from the request context
func setServiceInstanceCredentialUserInfo(instanceCredential *sc.ServiceInstanceCredential, ctx genericapirequest.Context) {
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
