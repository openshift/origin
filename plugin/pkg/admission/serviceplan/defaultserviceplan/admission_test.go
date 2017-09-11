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
	handler, err := NewDefaultServicePlan()
	if err != nil {
		return nil, f, err
	}
	pluginInitializer := scadmission.NewPluginInitializer(internalClient, f, nil, nil)
	pluginInitializer.Initialize(handler)
	err = admission.Validate(handler)
	return handler, f, err
}

// newFakeServiceCatalogClientForTest creates a fake clientset that returns a
// ServiceClassList with the given ServiceClass as the single list item.
func newFakeServiceCatalogClientForTest(sc *servicecatalog.ServiceClass) *fake.Clientset {
	fakeClient := &fake.Clientset{}

	scList := &servicecatalog.ServiceClassList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: "1",
		}}
	scList.Items = append(scList.Items, *sc)

	fakeClient.AddReactor("list", "serviceclasses", func(action core.Action) (bool, runtime.Object, error) {
		return true, scList, nil
	})
	return fakeClient
}

// newServiceInstance returns a new instance for the specified namespace.
func newServiceInstance(namespace string) servicecatalog.ServiceInstance {
	return servicecatalog.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "instance", Namespace: namespace},
	}
}

// newServiceClass returns a new instance with the specified plans.
func newServiceClass(name string, plans ...string) *servicecatalog.ServiceClass {
	sc := &servicecatalog.ServiceClass{ObjectMeta: metav1.ObjectMeta{Name: name}}
	for _, plan := range plans {
		sc.Plans = append(sc.Plans, servicecatalog.ServicePlan{Name: plan})
	}
	return sc
}

func TestWithListFailure(t *testing.T) {
	fakeClient := &fake.Clientset{}
	fakeClient.AddReactor("list", "serviceclasses", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("simulated test failure")
	})
	handler, informerFactory, err := newHandlerForTest(fakeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	instance := newServiceInstance("dummy")
	instance.Spec.ServiceClassName = "foo"

	err = handler.Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Create, nil))
	if err == nil {
		t.Errorf("unexpected success with no ServiceClasses.List succeeding")
	} else if !strings.Contains(err.Error(), "not yet ready to handle request") {
		t.Errorf("did not find expected error, got %q", err)
	}
}

func TestWithPlanWorks(t *testing.T) {
	fakeClient := newFakeServiceCatalogClientForTest(&servicecatalog.ServiceClass{})
	handler, informerFactory, err := newHandlerForTest(fakeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	instance := newServiceInstance("dummy")
	instance.Spec.ServiceClassName = "foo"
	instance.Spec.PlanName = "bar"

	err = handler.Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Create, nil))
	if err != nil {
		actions := ""
		for _, action := range fakeClient.Actions() {
			actions = actions + action.GetVerb() + ":" + action.GetResource().Resource + ":" + action.GetSubresource() + ", "
		}
		t.Errorf("unexpected error %q returned from admission handler: %v", err, actions)
	}
}

func TestWithNoPlanFailsWithNoServiceClass(t *testing.T) {
	fakeClient := newFakeServiceCatalogClientForTest(&servicecatalog.ServiceClass{})
	handler, informerFactory, err := newHandlerForTest(fakeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	instance := newServiceInstance("dummy")
	instance.Spec.ServiceClassName = "foo"

	err = handler.Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Create, nil))
	if err == nil {
		t.Errorf("unexpected success with no plan specified and no serviceclass existing")
	} else if !strings.Contains(err.Error(), "does not exist, can not figure") {
		t.Errorf("did not find expected error, got %q", err)
	}
}

func TestWithNoPlanWorksWithSinglePlan(t *testing.T) {
	sc := newServiceClass("foo", "bar")
	glog.V(4).Infof("Created Service as %+v", sc)
	fakeClient := newFakeServiceCatalogClientForTest(sc)
	handler, informerFactory, err := newHandlerForTest(fakeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	instance := newServiceInstance("dummy")
	instance.Spec.ServiceClassName = "foo"

	err = handler.Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Create, nil))
	if err != nil {
		actions := ""
		for _, action := range fakeClient.Actions() {
			actions = actions + action.GetVerb() + ":" + action.GetResource().Resource + ":" + action.GetSubresource() + ", "
		}
		t.Errorf("unexpected error %q returned from admission handler: %v", err, actions)
	}
	// Make sure the ServiceInstance has been mutated to include the service plan name
	if instance.Spec.PlanName != "bar" {
		t.Errorf("PlanName was not modified for the default plan")
	}
}

func TestWithNoPlanFailsWithMultiplePlans(t *testing.T) {
	sc := newServiceClass("foo", "bar", "baz")
	glog.V(4).Infof("Created Service as %+v", sc)
	fakeClient := newFakeServiceCatalogClientForTest(sc)
	handler, informerFactory, err := newHandlerForTest(fakeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	instance := newServiceInstance("dummy")
	instance.Spec.ServiceClassName = "foo"

	err = handler.Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Create, nil))
	if err == nil {
		t.Errorf("unexpected success with no plan specified and no serviceclass existing")
		return
	} else if !strings.Contains(err.Error(), "has more than one plan, PlanName must be") {
		t.Errorf("did not find expected error, got %q", err)
	}
}
