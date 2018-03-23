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

package clusterserviceclass

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

// NewScopeStrategy returns a new NamespaceScopedStrategy for cluster service
// classes.
func NewScopeStrategy() rest.NamespaceScopedStrategy {
	return clusterServiceClassRESTStrategies
}

// clusterServiceClassRESTStrategy implements interfaces RESTCreateStrategy,
// RESTUpdateStrategy, RESTDeleteStrategy, NamespaceScopedStrategy.
type clusterServiceClassRESTStrategy struct {
	runtime.ObjectTyper // inherit ObjectKinds method
	names.NameGenerator // GenerateName method for CreateStrategy
}

// clusterServiceClassStatusRESTStrategy implements interface
// RESTUpdateStrategy. This implementation validates updates to
// clusterServiceClass.Status updates only and disallows any modifications to
// the clusterServiceClass.Spec.
type clusterServiceClassStatusRESTStrategy struct {
	clusterServiceClassRESTStrategy
}

var (
	clusterServiceClassRESTStrategies = clusterServiceClassRESTStrategy{
		// embeds to pull in existing code behavior from upstream

		ObjectTyper: api.Scheme,
		// use the generator from upstream k8s, or implement method
		// `GenerateName(base string) string`
		NameGenerator: names.SimpleNameGenerator,
	}
	_ rest.RESTCreateStrategy = clusterServiceClassRESTStrategies
	_ rest.RESTUpdateStrategy = clusterServiceClassRESTStrategies
	_ rest.RESTDeleteStrategy = clusterServiceClassRESTStrategies

	clusterServiceClassStatusUpdateStrategy = clusterServiceClassStatusRESTStrategy{
		clusterServiceClassRESTStrategies,
	}
	_ rest.RESTUpdateStrategy = clusterServiceClassStatusUpdateStrategy
)

// Canonicalize does not transform a ClusterServiceClass.
func (clusterServiceClassRESTStrategy) Canonicalize(obj runtime.Object) {
	_, ok := obj.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-clusterserviceclass object to create")
	}
}

// NamespaceScoped returns false as ClusterServiceClass are not scoped to a
// namespace.
func (clusterServiceClassRESTStrategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate receives the incoming ClusterServiceClass.
func (clusterServiceClassRESTStrategy) PrepareForCreate(ctx genericapirequest.Context, obj runtime.Object) {
	clusterServiceClass, ok := obj.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-clusterserviceclass object to create")
	}
	clusterServiceClass.Status = sc.ClusterServiceClassStatus{}
}

func (clusterServiceClassRESTStrategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	return scv.ValidateClusterServiceClass(obj.(*sc.ClusterServiceClass))
}

func (clusterServiceClassRESTStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (clusterServiceClassRESTStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (clusterServiceClassRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newServiceClass, ok := new.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-clusterserviceclass object to update to")
	}
	oldServiceClass, ok := old.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-clusterserviceclass object to update from")
	}

	// Update should not change the status
	newServiceClass.Status = oldServiceClass.Status

	newServiceClass.Spec.ClusterServiceBrokerName = oldServiceClass.Spec.ClusterServiceBrokerName
}

func (clusterServiceClassRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	newServiceclass, ok := new.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-clusterserviceclass object to validate to")
	}
	oldServiceclass, ok := old.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-clusterserviceclass object to validate from")
	}

	return scv.ValidateClusterServiceClassUpdate(newServiceclass, oldServiceclass)
}

func (clusterServiceClassStatusRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newServiceClass, ok := new.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-clusterserviceClass object to update to")
	}
	oldServiceClass, ok := old.(*sc.ClusterServiceClass)
	if !ok {
		glog.Fatal("received a non-clusterserviceClass object to update from")
	}
	// Status changes are not allowed to update spec
	newServiceClass.Spec = oldServiceClass.Spec
}

func (clusterServiceClassStatusRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	return field.ErrorList{}
}
