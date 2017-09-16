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

package changevalidator

import (
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	scadmission "github.com/kubernetes-incubator/service-catalog/pkg/apiserver/admission"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/internalclientset"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/internalclientset/fake"
	informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/internalversion"
	core "k8s.io/client-go/testing"
)

// newHandlerForTest returns a configured handler for testing.
func newHandlerForTest(internalClient internalclientset.Interface) (admission.Interface, informers.SharedInformerFactory, error) {
	f := informers.NewSharedInformerFactory(internalClient, 5*time.Minute)
	handler, err := NewDenyPlanChangeIfNotUpdatable()
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
func newServiceInstance(namespace string, serviceClassName string, planName string) servicecatalog.ServiceInstance {
	instance := servicecatalog.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "instance", Namespace: namespace},
	}
	instance.Spec.ServiceClassName = serviceClassName
	instance.Spec.PlanName = planName
	return instance
}

// newServiceClass returns a new instance with the specified plan and
// UpdateablePlan attribute
func newServiceClass(name string, plan string, updateablePlan bool) *servicecatalog.ServiceClass {
	sc := &servicecatalog.ServiceClass{ObjectMeta: metav1.ObjectMeta{Name: name}, PlanUpdatable: updateablePlan}
	sc.Plans = append(sc.Plans, servicecatalog.ServicePlan{Name: plan})
	return sc
}

// setupInstanceLister creates a Service Instance and sets up a Instance Lister that
// retuns the instance
func setupInstanceLister(fakeClient *fake.Clientset) {
	instance := newServiceInstance("dummy", "foo", "original-plan-name")
	scList := &servicecatalog.ServiceInstanceList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: "1",
		}}
	scList.Items = append(scList.Items, instance)

	fakeClient.AddReactor("list", "serviceinstances", func(action core.Action) (bool, runtime.Object, error) {
		return true, scList, nil
	})
}

// TestServicePlanChangeBlockedByUpdateablePlanSetting tests that the
// Admission Controller will block a request to update an Instance's
// Service Plan
func TestServicePlanChangeBlockedByUpdateablePlanSetting(t *testing.T) {
	sc := newServiceClass("foo", "bar", false)
	fakeClient := newFakeServiceCatalogClientForTest(sc)
	handler, informerFactory, err := newHandlerForTest(fakeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	setupInstanceLister(fakeClient)
	instance := newServiceInstance("dummy", "foo", "new-plan")
	informerFactory.Start(wait.NeverStop)
	err = handler.Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Update, nil))
	if err != nil {
		if !strings.Contains(err.Error(), "The Service Class foo does not allow plan changes.") {
			t.Errorf("unexpected error %q returned from admission handler.", err.Error())
		}
	} else {
		t.Error("This should have been an error")
	}
}

// TestServicePlanChangePermittedByUpdateablePlanSetting tests the
// Admission Controller verifying it allows an instance change to the
// plan name if the service class has specified PlanUpdatable=true
func TestServicePlanChangePermittedByUpdateablePlanSetting(t *testing.T) {
	sc := newServiceClass("foo", "bar", true)
	fakeClient := newFakeServiceCatalogClientForTest(sc)
	handler, informerFactory, err := newHandlerForTest(fakeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}

	setupInstanceLister(fakeClient)

	instance := newServiceInstance("dummy", "foo", "new-plan")
	informerFactory.Start(wait.NeverStop)
	err = handler.Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Update, nil))
	if err != nil {
		t.Errorf("Unexpected error: %v", err.Error())
	}
}
