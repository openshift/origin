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

package instance

// this was copied from where else and edited to fit our objects

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	api "k8s.io/client-go/pkg/api"

	"github.com/golang/glog"
	sc "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	checksum "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/checksum/unversioned"
	scv "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/validation"
)

// NewScopeStrategy returns a new NamespaceScopedStrategy for instances
func NewScopeStrategy() rest.NamespaceScopedStrategy {
	return instanceRESTStrategies
}

// implements interfaces RESTCreateStrategy, RESTUpdateStrategy, RESTDeleteStrategy,
// NamespaceScopedStrategy
// The implementation disallows any modifications to the instance.Status fields.
type instanceRESTStrategy struct {
	runtime.ObjectTyper // inherit ObjectKinds method
	names.NameGenerator // GenerateName method for CreateStrategy
}

// implements interface RESTUpdateStrategy. This implementation validates updates to
// instance.Status updates only and disallows any modifications to the instance.Spec.
type instanceStatusRESTStrategy struct {
	instanceRESTStrategy
}

var (
	instanceRESTStrategies = instanceRESTStrategy{
		// embeds to pull in existing code behavior from upstream

		ObjectTyper: api.Scheme,
		// use the generator from upstream k8s, or implement method
		// `GenerateName(base string) string`
		NameGenerator: names.SimpleNameGenerator,
	}
	_ rest.RESTCreateStrategy = instanceRESTStrategies
	_ rest.RESTUpdateStrategy = instanceRESTStrategies
	_ rest.RESTDeleteStrategy = instanceRESTStrategies

	instanceStatusUpdateStrategy = instanceStatusRESTStrategy{
		instanceRESTStrategies,
	}
	_ rest.RESTUpdateStrategy = instanceStatusUpdateStrategy
)

// Canonicalize does not transform a instance.
func (instanceRESTStrategy) Canonicalize(obj runtime.Object) {
	_, ok := obj.(*sc.ServiceInstance)
	if !ok {
		glog.Fatal("received a non-instance object to create")
	}
}

// NamespaceScoped returns false as instances are not scoped to a namespace.
func (instanceRESTStrategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate receives a the incoming ServiceInstance and clears it's
// Status. Status is not a user settable field.
func (instanceRESTStrategy) PrepareForCreate(ctx genericapirequest.Context, obj runtime.Object) {
	instance, ok := obj.(*sc.ServiceInstance)
	if !ok {
		glog.Fatal("received a non-instance object to create")
	}

	// Creating a brand new object, thus it must have no
	// status. We can't fail here if they passed a status in, so
	// we just wipe it clean.
	instance.Status = sc.ServiceInstanceStatus{}
	// Fill in the first entry set to "creating"?
	instance.Status.Conditions = []sc.ServiceInstanceCondition{}
	instance.Finalizers = []string{sc.FinalizerServiceCatalog}
}

func (instanceRESTStrategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	return scv.ValidateServiceInstance(obj.(*sc.ServiceInstance))
}

func (instanceRESTStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (instanceRESTStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (instanceRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newServiceInstance, ok := new.(*sc.ServiceInstance)
	if !ok {
		glog.Fatal("received a non-instance object to update to")
	}
	oldServiceInstance, ok := old.(*sc.ServiceInstance)
	if !ok {
		glog.Fatal("received a non-instance object to update from")
	}

	// TODO: We currently don't handle any changes to the spec in the
	// reconciler. Once we do that, this check needs to be removed and
	// proper validation of allowed changes needs to be implemented in
	// ValidateUpdate
	newServiceInstance.Spec = oldServiceInstance.Spec

	// Do not allow any updates to the Status field while updating the Spec
	newServiceInstance.Status = oldServiceInstance.Status
}

func (instanceRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	newServiceInstance, ok := new.(*sc.ServiceInstance)
	if !ok {
		glog.Fatal("received a non-instance object to validate to")
	}
	oldServiceInstance, ok := old.(*sc.ServiceInstance)
	if !ok {
		glog.Fatal("received a non-instance object to validate from")
	}

	return scv.ValidateServiceInstanceUpdate(newServiceInstance, oldServiceInstance)
}

func (instanceStatusRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newServiceInstance, ok := new.(*sc.ServiceInstance)
	if !ok {
		glog.Fatal("received a non-instance object to update to")
	}
	oldServiceInstance, ok := old.(*sc.ServiceInstance)
	if !ok {
		glog.Fatal("received a non-instance object to update from")
	}
	// Status changes are not allowed to update spec
	newServiceInstance.Spec = oldServiceInstance.Spec

	foundReadyConditionTrue := false
	for _, condition := range newServiceInstance.Status.Conditions {
		if condition.Type == sc.ServiceInstanceConditionReady && condition.Status == sc.ConditionTrue {
			foundReadyConditionTrue = true
			break
		}
	}

	if foundReadyConditionTrue {
		glog.Infof("Found true ready condition for ServiceInstance %v/%v; updating checksum", newServiceInstance.Namespace, newServiceInstance.Name)
		// This status update has a true ready condition; update the checksum
		// if necessary
		newServiceInstance.Status.Checksum = func() *string {
			s := checksum.ServiceInstanceSpecChecksum(newServiceInstance.Spec)
			return &s
		}()
		return
	}

	// if the ready condition is not true, the value of the checksum should
	// not change.
	newServiceInstance.Status.Checksum = oldServiceInstance.Status.Checksum
}

func (instanceStatusRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	newServiceInstance, ok := new.(*sc.ServiceInstance)
	if !ok {
		glog.Fatal("received a non-instance object to validate to")
	}
	oldServiceInstance, ok := old.(*sc.ServiceInstance)
	if !ok {
		glog.Fatal("received a non-instance object to validate from")
	}

	return scv.ValidateServiceInstanceStatusUpdate(newServiceInstance, oldServiceInstance)
}
