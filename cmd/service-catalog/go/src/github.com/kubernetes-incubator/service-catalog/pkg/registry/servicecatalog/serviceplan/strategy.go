/*
Copyright 2016 The Kubernetes Authors.

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

package serviceplan

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

// NewScopeStrategy returns a new NamespaceScopedStrategy for service planes
func NewScopeStrategy() rest.NamespaceScopedStrategy {
	return serviceplanRESTStrategies
}

// implements interfaces RESTCreateStrategy, RESTUpdateStrategy, RESTDeleteStrategy,
// NamespaceScopedStrategy
type serviceplanRESTStrategy struct {
	runtime.ObjectTyper // inherit ObjectKinds method
	names.NameGenerator // GenerateName method for CreateStrategy
}

var (
	serviceplanRESTStrategies = serviceplanRESTStrategy{
		// embeds to pull in existing code behavior from upstream

		ObjectTyper: api.Scheme,
		// use the generator from upstream k8s, or implement method
		// `GenerateName(base string) string`
		NameGenerator: names.SimpleNameGenerator,
	}
	_ rest.RESTCreateStrategy = serviceplanRESTStrategies
	_ rest.RESTUpdateStrategy = serviceplanRESTStrategies
	_ rest.RESTDeleteStrategy = serviceplanRESTStrategies
)

// Canonicalize does not transform a serviceplan.
func (serviceplanRESTStrategy) Canonicalize(obj runtime.Object) {
	_, ok := obj.(*sc.ServicePlan)
	if !ok {
		glog.Fatal("received a non-serviceplan object to create")
	}
}

// NamespaceScoped returns false as serviceplans are not scoped to a namespace.
func (serviceplanRESTStrategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate receives the incoming Serviceplan.
func (serviceplanRESTStrategy) PrepareForCreate(ctx genericapirequest.Context, obj runtime.Object) {
	_, ok := obj.(*sc.ServicePlan)
	if !ok {
		glog.Fatal("received a non-serviceplan object to create")
	}
	// service plan is a data record and has no status to track
}

func (serviceplanRESTStrategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	return scv.ValidateServicePlan(obj.(*sc.ServicePlan))
}

func (serviceplanRESTStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (serviceplanRESTStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (serviceplanRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newServiceplan, ok := new.(*sc.ServicePlan)
	if !ok {
		glog.Fatal("received a non-serviceplan object to update to")
	}
	oldServiceplan, ok := old.(*sc.ServicePlan)
	if !ok {
		glog.Fatal("received a non-serviceplan object to update from")
	}
	// copy all fields individually?
	newServiceplan.Spec.ServiceClassRef = oldServiceplan.Spec.ServiceClassRef
}

func (serviceplanRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	newServiceplan, ok := new.(*sc.ServicePlan)
	if !ok {
		glog.Fatal("received a non-serviceplan object to validate to")
	}
	oldServiceplan, ok := old.(*sc.ServicePlan)
	if !ok {
		glog.Fatal("received a non-serviceplan object to validate from")
	}

	return scv.ValidateServicePlanUpdate(newServiceplan, oldServiceplan)
}
