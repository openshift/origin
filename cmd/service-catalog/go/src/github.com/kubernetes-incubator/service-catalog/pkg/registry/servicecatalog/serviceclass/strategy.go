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

package serviceclass

// this was copied from where else and edited to fit our objects

import (
	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/golang/glog"
	sc "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	scv "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/validation"
)

// NewScopeStrategy returns a new NamespaceScopedStrategy for service classes
func NewScopeStrategy() rest.NamespaceScopedStrategy {
	return serviceClassRESTStrategies
}

// implements interfaces RESTCreateStrategy, RESTUpdateStrategy, RESTDeleteStrategy,
// NamespaceScopedStrategy
type serviceClassRESTStrategy struct {
	runtime.ObjectTyper // inherit ObjectKinds method
	names.NameGenerator // GenerateName method for CreateStrategy
}

// implements interface RESTUpdateStrategy. This implementation validates updates to
// serviceClass.Status updates only and disallows any modifications to the serviceClass.Spec.
type serviceClassStatusRESTStrategy struct {
	serviceClassRESTStrategy
}

var (
	serviceClassRESTStrategies = serviceClassRESTStrategy{
		// embeds to pull in existing code behavior from upstream

		ObjectTyper: api.Scheme,
		// use the generator from upstream k8s, or implement method
		// `GenerateName(base string) string`
		NameGenerator: names.SimpleNameGenerator,
	}
	_ rest.RESTCreateStrategy = serviceClassRESTStrategies
	_ rest.RESTUpdateStrategy = serviceClassRESTStrategies
	_ rest.RESTDeleteStrategy = serviceClassRESTStrategies

	serviceClassStatusUpdateStrategy = serviceClassStatusRESTStrategy{
		serviceClassRESTStrategies,
	}
	_ rest.RESTUpdateStrategy = serviceClassStatusUpdateStrategy
)

// Canonicalize does not transform a serviceclass.
func (serviceClassRESTStrategy) Canonicalize(obj runtime.Object) {
	_, ok := obj.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-serviceclass object to create")
	}
}

// NamespaceScoped returns false as serviceclasss are not scoped to a namespace.
func (serviceClassRESTStrategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate receives the incoming Serviceclass.
func (serviceClassRESTStrategy) PrepareForCreate(ctx genericapirequest.Context, obj runtime.Object) {
	serviceClass, ok := obj.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-serviceclass object to create")
	}
	serviceClass.Status = sc.ClusterServiceClassStatus{}
}

func (serviceClassRESTStrategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	return scv.ValidateClusterServiceClass(obj.(*sc.ClusterServiceClass))
}

func (serviceClassRESTStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (serviceClassRESTStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (serviceClassRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newServiceClass, ok := new.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-serviceclass object to update to")
	}
	oldServiceClass, ok := old.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-serviceclass object to update from")
	}

	// Update should not change the status
	newServiceClass.Status = oldServiceClass.Status

	newServiceClass.Spec.ClusterServiceBrokerName = oldServiceClass.Spec.ClusterServiceBrokerName
}

func (serviceClassRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	newServiceclass, ok := new.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-serviceclass object to validate to")
	}
	oldServiceclass, ok := old.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-serviceclass object to validate from")
	}

	return scv.ValidateClusterServiceClassUpdate(newServiceclass, oldServiceclass)
}

func (serviceClassStatusRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newServiceClass, ok := new.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-serviceClass object to update to")
	}
	oldServiceClass, ok := old.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-serviceClass object to update from")
	}
	// Status changes are not allowed to update spec
	newServiceClass.Spec = oldServiceClass.Spec
}

func (serviceClassStatusRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	// newServiceClass, ok := new.(*sc.ServiceClass)
	// if !ok {
	// 	glog.Fatal("received a non-serviceClass object to validate to")
	// }
	// oldServiceClass, ok := old.(*sc.ServiceClass)
	// if !ok {
	// 	glog.Fatal("received a non-serviceClass object to validate from")
	// }

	// return scv.ValidateServiceClassStatusUpdate(newServiceClass, oldServiceClass)

	return field.ErrorList{}
}
