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

package broker

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

// NewScopeStrategy returns a new NamespaceScopedStrategy for brokers
func NewScopeStrategy() rest.NamespaceScopedStrategy {
	return brokerRESTStrategies
}

// implements interfaces RESTCreateStrategy, RESTUpdateStrategy, RESTDeleteStrategy,
// NamespaceScopedStrategy
type brokerRESTStrategy struct {
	runtime.ObjectTyper // inherit ObjectKinds method
	names.NameGenerator // GenerateName method for CreateStrategy
}

// implements interface RESTUpdateStrategy
type brokerStatusRESTStrategy struct {
	brokerRESTStrategy
}

var (
	brokerRESTStrategies = brokerRESTStrategy{
		// embeds to pull in existing code behavior from upstream

		// this has an interesting NOTE on it. Not sure if it applies to us.
		ObjectTyper: api.Scheme,
		// use the generator from upstream k8s, or implement method
		// `GenerateName(base string) string`
		NameGenerator: names.SimpleNameGenerator,
	}
	_ rest.RESTCreateStrategy = brokerRESTStrategies
	_ rest.RESTUpdateStrategy = brokerRESTStrategies
	_ rest.RESTDeleteStrategy = brokerRESTStrategies

	brokerStatusUpdateStrategy = brokerStatusRESTStrategy{
		brokerRESTStrategies,
	}
	_ rest.RESTUpdateStrategy = brokerStatusUpdateStrategy
)

// Canonicalize does not transform a broker.
func (brokerRESTStrategy) Canonicalize(obj runtime.Object) {
	_, ok := obj.(*sc.Broker)
	if !ok {
		glog.Fatal("received a non-broker object to create")
	}
}

// NamespaceScoped returns false as brokers are not scoped to a namespace.
func (brokerRESTStrategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate receives a the incoming Broker and clears it's
// Status. Status is not a user settable field.
func (brokerRESTStrategy) PrepareForCreate(ctx genericapirequest.Context, obj runtime.Object) {
	broker, ok := obj.(*sc.Broker)
	if !ok {
		glog.Fatal("received a non-broker object to create")
	}
	// Is there anything to pull out of the context `ctx`?

	// Creating a brand new object, thus it must have no
	// status. We can't fail here if they passed a status in, so
	// we just wipe it clean.
	broker.Status = sc.BrokerStatus{}
	// Fill in the first entry set to "creating"?
	broker.Status.Conditions = []sc.BrokerCondition{}
	broker.Finalizers = []string{sc.FinalizerServiceCatalog}
}

func (brokerRESTStrategy) Validate(ctx genericapirequest.Context, obj runtime.Object) field.ErrorList {
	return scv.ValidateBroker(obj.(*sc.Broker))
}

func (brokerRESTStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (brokerRESTStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (brokerRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newBroker, ok := new.(*sc.Broker)
	if !ok {
		glog.Fatal("received a non-broker object to update to")
	}
	oldBroker, ok := old.(*sc.Broker)
	if !ok {
		glog.Fatal("received a non-broker object to update from")
	}

	newBroker.Status = oldBroker.Status
}

func (brokerRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	newBroker, ok := new.(*sc.Broker)
	if !ok {
		glog.Fatal("received a non-broker object to validate to")
	}
	oldBroker, ok := old.(*sc.Broker)
	if !ok {
		glog.Fatal("received a non-broker object to validate from")
	}

	return scv.ValidateBrokerUpdate(newBroker, oldBroker)
}

func (brokerStatusRESTStrategy) PrepareForUpdate(ctx genericapirequest.Context, new, old runtime.Object) {
	newBroker, ok := new.(*sc.Broker)
	if !ok {
		glog.Fatal("received a non-broker object to update to")
	}
	oldBroker, ok := old.(*sc.Broker)
	if !ok {
		glog.Fatal("received a non-broker object to update from")
	}
	// status changes are not allowed to update spec
	newBroker.Spec = oldBroker.Spec

	for _, condition := range newBroker.Status.Conditions {
		if condition.Type == sc.BrokerConditionReady && condition.Status == sc.ConditionTrue {
			glog.Infof("Found true ready condition for Broker %v/%v; updating checksum", newBroker.Namespace, newBroker.Name)
			newBroker.Status.Checksum = func() *string {
				s := checksum.BrokerSpecChecksum(newBroker.Spec)
				return &s
			}()
			return
		}
	}

	// if the ready condition is not true, the value of the checksum should
	// not change.
	newBroker.Status.Checksum = oldBroker.Status.Checksum
}

func (brokerStatusRESTStrategy) ValidateUpdate(ctx genericapirequest.Context, new, old runtime.Object) field.ErrorList {
	newBroker, ok := new.(*sc.Broker)
	if !ok {
		glog.Fatal("received a non-broker object to validate to")
	}
	oldBroker, ok := old.(*sc.Broker)
	if !ok {
		glog.Fatal("received a non-broker object to validate from")
	}

	return scv.ValidateBrokerStatusUpdate(newBroker, oldBroker)
}
