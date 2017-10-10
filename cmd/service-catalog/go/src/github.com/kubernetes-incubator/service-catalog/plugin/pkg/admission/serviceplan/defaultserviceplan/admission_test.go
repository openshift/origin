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

package defaultserviceplan

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"
	core "k8s.io/client-go/testing"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	scadmission "github.com/kubernetes-incubator/service-catalog/pkg/apiserver/admission"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/internalclientset"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/internalclientset/fake"
	informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/internalversion"
)

// newHandlerForTest returns a configured handler for testing.
func newHandlerForTest(internalClient internalclientset.Interface) (admission.Interface, informers.SharedInformerFactory, error) {
	f := informers.NewSharedInformerFactory(internalClient, 5*time.Minute)
	handler, err := NewDefaultClusterServicePlan()
	if err != nil {
		return nil, f, err
	}
	pluginInitializer := scadmission.NewPluginInitializer(internalClient, f, nil, nil)
	pluginInitializer.Initialize(handler)
	err = admission.Validate(handler)
	return handler, f, err
}

// newFakeServiceCatalogClientForTest creates a fake clientset that returns a
// ClusterServiceClassList with the given ClusterServiceClass as the single list item.
func newFakeServiceCatalogClientForTest(sc *servicecatalog.ClusterServiceClass, sps []*servicecatalog.ClusterServicePlan) *fake.Clientset {
	fakeClient := &fake.Clientset{}

	// react to the given service classes
	scList := &servicecatalog.ClusterServiceClassList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: "1",
		}}
	if sc != nil {
		scList.Items = append(scList.Items, *sc)
	}

	fakeClient.AddReactor("list", "clusterserviceclasses", func(action core.Action) (bool, runtime.Object, error) {
		return true, scList, nil
	})

	// react to the given plans
	spList := &servicecatalog.ClusterServicePlanList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: "1",
		}}
	for _, sp := range sps {
		spList.Items = append(spList.Items, *sp)
	}
	fakeClient.AddReactor("list", "clusterserviceplans", func(action core.Action) (bool, runtime.Object, error) {
		return true, spList, nil
	})

	return fakeClient
}

// newServiceInstance returns a new instance for the specified namespace.
func newServiceInstance(namespace string) servicecatalog.ServiceInstance {
	return servicecatalog.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "instance", Namespace: namespace},
	}
}

// newClusterServiceClass returns a new serviceclass.
func newClusterServiceClass(id string, name string) *servicecatalog.ClusterServiceClass {
	sc := &servicecatalog.ClusterServiceClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: id,
		},
		Spec: servicecatalog.ClusterServiceClassSpec{
			ExternalID:   id,
			ExternalName: name,
		},
	}
	return sc
}

// newClusterServiceClass returns a new serviceclass.
func newClusterServicePlans(count uint) []*servicecatalog.ClusterServicePlan {
	sp1 := &servicecatalog.ClusterServicePlan{
		ObjectMeta: metav1.ObjectMeta{Name: "bar-id"},
		Spec: servicecatalog.ClusterServicePlanSpec{
			ExternalName: "bar",
			ExternalID:   "12345",
		},
	}

	sp2 := &servicecatalog.ClusterServicePlan{
		ObjectMeta: metav1.ObjectMeta{Name: "baz-id"},
		Spec: servicecatalog.ClusterServicePlanSpec{
			ExternalName: "baz",
			ExternalID:   "23456",
		},
	}

	if 0 == count {
		return []*servicecatalog.ClusterServicePlan{}
	}
	if 1 == count {
		return []*servicecatalog.ClusterServicePlan{sp1}
	}
	if 2 == count {
		return []*servicecatalog.ClusterServicePlan{sp1, sp2}
	}
	return []*servicecatalog.ClusterServicePlan{}
}

func TestWithListFailure(t *testing.T) {
	fakeClient := &fake.Clientset{}
	fakeClient.AddReactor("list", "clusterserviceclasses", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("simulated test failure")
	})
	handler, informerFactory, err := newHandlerForTest(fakeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	instance := newServiceInstance("dummy")
	instance.Spec.ExternalClusterServiceClassName = "foo"

	err = handler.Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Create, nil))
	if err == nil {
		t.Errorf("unexpected success with no ClusterServiceClasses.List succeeding")
	} else if !strings.Contains(err.Error(), "simulated test failure") {
		t.Errorf("did not find expected error, got %q", err)
	}
}

func TestWithPlanWorks(t *testing.T) {
	fakeClient := newFakeServiceCatalogClientForTest(nil, newClusterServicePlans(1))
	handler, informerFactory, err := newHandlerForTest(fakeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	instance := newServiceInstance("dummy")
	instance.Spec.ExternalClusterServiceClassName = "foo"
	instance.Spec.ExternalClusterServicePlanName = "bar"

	err = handler.Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Create, nil))
	if err != nil {
		actions := ""
		for _, action := range fakeClient.Actions() {
			actions = actions + action.GetVerb() + ":" + action.GetResource().Resource + ":" + action.GetSubresource() + ", "
		}
		t.Errorf("unexpected error %q returned from admission handler: %v", err, actions)
	}
}

func TestWithNoPlanFailsWithNoClusterServiceClass(t *testing.T) {
	fakeClient := newFakeServiceCatalogClientForTest(nil, newClusterServicePlans(1))
	handler, informerFactory, err := newHandlerForTest(fakeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	instance := newServiceInstance("dummy")
	instance.Spec.ExternalClusterServiceClassName = "foobar"

	err = handler.Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Create, nil))
	if err == nil {
		t.Errorf("unexpected success with no plan specified and no serviceclass existing")
	} else if !strings.Contains(err.Error(), "does not exist, can not figure") {
		t.Errorf("did not find expected error, got %q", err)
	}
}

// checks that the defaulting action works when a service class only provides a single plan.
func TestWithNoPlanWorksWithSinglePlan(t *testing.T) {
	sc := newClusterServiceClass("foo-id", "foo")
	sps := newClusterServicePlans(1)
	glog.V(4).Infof("Created Service as %+v", sc)
	fakeClient := newFakeServiceCatalogClientForTest(sc, sps)

	handler, informerFactory, err := newHandlerForTest(fakeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	instance := newServiceInstance("dummy")
	instance.Spec.ExternalClusterServiceClassName = "foo"

	err = handler.Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Create, nil))
	if err != nil {
		actions := ""
		for _, action := range fakeClient.Actions() {
			actions = actions + action.GetVerb() + ":" + action.GetResource().Resource + ":" + action.GetSubresource() + ", "
		}
		t.Errorf("unexpected error %q returned from admission handler: %v", err, actions)
	}
	// Make sure the ServiceInstance has been mutated to include the service plan name
	if instance.Spec.ExternalClusterServicePlanName != "bar" {
		t.Errorf("PlanName was not modified for the default plan")
	}
}

// checks that defaulting fails when there are multiple plans to choose from.
func TestWithNoPlanFailsWithMultiplePlans(t *testing.T) {
	sc := newClusterServiceClass("foo-id", "foo")
	sps := newClusterServicePlans(2)
	glog.V(4).Infof("Created Service as %+v", sc)
	fakeClient := newFakeServiceCatalogClientForTest(sc, sps)
	handler, informerFactory, err := newHandlerForTest(fakeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	instance := newServiceInstance("dummy")
	instance.Spec.ExternalClusterServiceClassName = "foo"

	err = handler.Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Create, nil))
	if err == nil {
		t.Errorf("unexpected success with no plan specified and no serviceclass existing")
		return
	} else if !strings.Contains(err.Error(), "has more than one plan, PlanName must be") {
		t.Errorf("did not find expected error, got %q", err)
	}
}
