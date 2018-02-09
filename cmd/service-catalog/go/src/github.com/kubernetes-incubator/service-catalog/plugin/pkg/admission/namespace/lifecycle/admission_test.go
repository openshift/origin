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

package lifecycle

import (
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"
	kubefake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"

	kubeinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/internalclientset"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/internalclientset/fake"
	informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/internalversion"

	scadmission "github.com/kubernetes-incubator/service-catalog/pkg/apiserver/admission"
)

// newHandlerForTest returns a configured handler for testing.
func newHandlerForTest(internalClient internalclientset.Interface, kubeClient kubeclientset.Interface) (admission.Interface, informers.SharedInformerFactory, kubeinformers.SharedInformerFactory, error) {
	f := informers.NewSharedInformerFactory(internalClient, 5*time.Minute)
	kf := kubeinformers.NewSharedInformerFactory(kubeClient, 5*time.Minute)
	handler, err := NewLifecycle()
	if err != nil {
		return nil, f, kf, err
	}
	pluginInitializer := scadmission.NewPluginInitializer(internalClient, f, kubeClient, kf)
	pluginInitializer.Initialize(handler)
	err = admission.ValidateInitialization(handler)
	return handler, f, kf, err
}

// newMockKubeClientForTest creates a mock client that returns a client
// configured for the specified list of namespaces with the specified phase.
func newMockKubeClientForTest(namespaces map[string]corev1.NamespacePhase) *kubefake.Clientset {
	mockClient := &kubefake.Clientset{}
	mockClient.AddReactor("list", "namespaces", func(action core.Action) (bool, runtime.Object, error) {
		namespaceList := &corev1.NamespaceList{
			ListMeta: metav1.ListMeta{
				ResourceVersion: fmt.Sprintf("%d", len(namespaces)),
			},
		}
		index := 0
		for name, phase := range namespaces {
			namespaceList.Items = append(namespaceList.Items, corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					ResourceVersion: fmt.Sprintf("%d", index),
				},
				Status: corev1.NamespaceStatus{
					Phase: phase,
				},
			})
			index++
		}
		return true, namespaceList, nil
	})
	return mockClient
}

// newMockClientForTest creates a mock client.
func newMockClientForTest() *fake.Clientset {
	mockClient := &fake.Clientset{}
	return mockClient
}

// newServiceInstance returns a new instance for the specified namespace.
func newServiceInstance(namespace string) servicecatalog.ServiceInstance {
	return servicecatalog.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "instance", Namespace: namespace},
	}
}

// TestAdmissionNamespaceDoesNotExist verifies instance is not admitted if namespace does not exist.
func TestAdmissionNamespaceDoesNotExist(t *testing.T) {
	namespace := "test"
	mockClient := newMockClientForTest()
	mockKubeClient := newMockKubeClientForTest(map[string]corev1.NamespacePhase{})
	mockKubeClient.AddReactor("get", "namespaces", func(action core.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("nope, out of luck")
	})
	handler, informerFactory, kubeInformerFactory, err := newHandlerForTest(mockClient, mockKubeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)
	kubeInformerFactory.Start(wait.NeverStop)

	instance := newServiceInstance(namespace)
	err = handler.(admission.MutationInterface).Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Create, nil))
	if err == nil {
		actions := ""
		for _, action := range mockClient.Actions() {
			actions = actions + action.GetVerb() + ":" + action.GetResource().Resource + ":" + action.GetSubresource() + ", "
		}
		t.Errorf("expected error returned from admission handler: %v", actions)
	}
}

// TestAdmissionNamespaceActive verifies a resource is admitted when the namespace is active.
func TestAdmissionNamespaceActive(t *testing.T) {
	namespace := "test"
	mockClient := newMockClientForTest()
	mockKubeClient := newMockKubeClientForTest(map[string]corev1.NamespacePhase{
		namespace: corev1.NamespaceActive,
	})
	handler, informerFactory, kubeInformerFactory, err := newHandlerForTest(mockClient, mockKubeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)
	kubeInformerFactory.Start(wait.NeverStop)

	instance := newServiceInstance(namespace)
	err = handler.(admission.MutationInterface).Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Create, nil))
	if err != nil {
		t.Errorf("unexpected error returned from admission handler")
	}
}

// TestAdmissionNamespaceTerminating verifies a resource is not created when the namespace is terminating.
func TestAdmissionNamespaceTerminating(t *testing.T) {
	namespace := "test"
	mockClient := newMockClientForTest()
	mockKubeClient := newMockKubeClientForTest(map[string]corev1.NamespacePhase{
		namespace: corev1.NamespaceTerminating,
	})
	handler, informerFactory, kubeInformerFactory, err := newHandlerForTest(mockClient, mockKubeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)
	kubeInformerFactory.Start(wait.NeverStop)

	instance := newServiceInstance(namespace)
	err = handler.(admission.MutationInterface).Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Create, nil))
	if err == nil {
		t.Errorf("Expected error rejecting creates in a namespace when it is terminating")
	}

	// verify update operations in the namespace can proceed
	err = handler.(admission.MutationInterface).Admit(admission.NewAttributesRecord(&instance, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Update, nil))
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}

	// verify delete operations in the namespace can proceed
	err = handler.(admission.MutationInterface).Admit(admission.NewAttributesRecord(nil, nil, servicecatalog.Kind("ServiceInstance").WithVersion("version"), instance.Namespace, instance.Name, servicecatalog.Resource("serviceinstances").WithVersion("version"), "", admission.Delete, nil))
	if err != nil {
		t.Errorf("Unexpected error returned from admission handler: %v", err)
	}
}
